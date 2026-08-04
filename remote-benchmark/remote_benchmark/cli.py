import asyncio
import io
import itertools
import math
import os
import sys
import time
from pathlib import Path

import backoff
import click
import requests
import ujson
import web3
import yaml
from hexbytes import HexBytes

from .compare import (
    build_comparison,
    load_record,
    render_comparison_text,
    write_comparison_html,
)
from .config import load_config
from .libp2p import bootstrap_peers
from .monitor import BlockSTMMonitor, MempoolMonitor
from .preflight import (
    peer_connectivity_matrix,
    probe_peers,
    resolved_mempool_types,
    unreachable_nodes,
)
from .results import (
    build_aggregate_record,
    build_run_record,
    check_divergence,
    consensus_health_reasons,
    consensus_health_warnings,
    divergence_reasons,
    divergence_warnings,
    evaluate_saturation,
    write_run_record,
)
from .resources import fetch_node_exporter, scrape_disk_net_raw
from .soak import CheckpointSampler, fit_trends, soak_verdict
from .sweep import load_matrix, run_sweep, summarize_sweep
from .stats import (
    _fetch_prometheus,
    dump_block_stats,
    dump_eth_block_stats,
    scrape_consensus_health_raw,
    scrape_consensus_raw,
)
from .transaction import (
    EthTx,
    build_cosmos_tx,
    gen,
    json_rpc_send_body,
    physical_account_range,
    send_round_robin,
)
from .utils import (
    Tee,
    block_eth,
    block_height,
    block_txs,
    eth_block_number,
    gen_account,
    split_batch,
)

# reserved for the funding account, index 0 is the funder itself.
FUND_ACCOUNT_INDEX = 0
LOAD_COMMIT_TIMEOUT = 120
PROGRESS_INTERVAL_S = 3


def _tx_options(cfg) -> dict:
    return {
        "gas_price": cfg.gas_price,
        "chain_id": cfg.chain_id,
        "mix": cfg.mix_weights,
    }


def _wait_for_committed(
    get_height, count_txs, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT
):
    """Extend the sample until `expected_txs` have been counted committed.

    ``start`` is the pre-send anchor and can still contain setup traffic,
    so only count txs committed after it.
    """
    next_height = start + 1
    committed_txs = 0
    deadline = time.monotonic() + timeout
    started = time.monotonic()
    last_log = started

    while True:
        while next_height <= end:
            committed_txs += count_txs(next_height)
            next_height += 1
            if committed_txs >= expected_txs:
                return end, committed_txs

        now = time.monotonic()
        if now - last_log >= PROGRESS_INTERVAL_S:
            print(
                f"waiting for commits: height={next_height - 1} "
                f"committed={committed_txs}/{expected_txs}",
                file=sys.stderr,
            )
            last_log = now

        if now >= deadline:
            return end, committed_txs

        current = get_height()
        if current > end:
            end = current
        else:
            time.sleep(0.2)


def wait_for_committed_txs(rpc, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT):
    """Extend the sample until all generated Cosmos txs are committed."""
    return _wait_for_committed(
        lambda: block_height(rpc),
        lambda height: len(block_txs(height, rpc) or []),
        start,
        end,
        expected_txs,
        timeout,
    )


def wait_for_committed_eth_txs(
    json_rpc, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT
):
    """Extend the sample until all generated Ethereum txs are committed."""
    return _wait_for_committed(
        lambda: eth_block_number(json_rpc),
        lambda height: len(block_eth(height, json_rpc)["transactions"]),
        start,
        end,
        expected_txs,
        timeout,
    )


def current_sender_nonce(cfg, start, end, num_txs=None):
    """Return the shared current nonce for the benchmark's physical senders.

    `num_txs` overrides `cfg.num_txs` for callers that generate a different
    per-account tx count (the soak derives its own from rate x duration): under
    the unique-per-tx strategy that count sets how wide the physical sender
    range is, so checking nonces with the config's value would validate a
    different set of accounts than the run actually signs from.
    """
    physical_start, physical_end = physical_account_range(
        start, end, cfg.num_txs if num_txs is None else num_txs, cfg.sender_strategy
    )
    w3 = web3.Web3(web3.HTTPProvider(cfg.primary.json_rpc))
    nonces = {
        w3.eth.get_transaction_count(gen_account(cfg.global_seq, i).address)
        for i in range(physical_start, physical_end + 1)
    }
    if len(nonces) != 1:
        values = ", ".join(str(value) for value in sorted(nonces))
        raise click.ClickException(
            f"benchmark sender accounts have different nonces ({values}); "
            "pass --nonce explicitly or use a fresh account range"
        )
    return nonces.pop()


@click.group()
def cli():
    pass


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--batch-size", default=200)
@click.option("--mode", "fund_mode", type=click.Choice(["cosmos", "eth"]))
@click.argument("start", type=int)
@click.argument("end", type=int)
def fund(config_path, batch_size, fund_mode, start, end):
    """Fund generated test accounts [start, end] from the funding account."""
    cfg = load_config(config_path)
    start, end = physical_account_range(start, end, cfg.num_txs, cfg.sender_strategy)
    fund_mode = fund_mode or cfg.mode
    w3 = web3.Web3(web3.HTTPProvider(cfg.primary.json_rpc))
    fund_account = gen_account(cfg.global_seq, FUND_ACCOUNT_INDEX)
    fund_address = HexBytes(fund_account.address)
    nonce = w3.eth.get_transaction_count(fund_account.address)

    for begin, chunk_end in split_batch(end - start + 1, batch_size):
        begin += start
        chunk_end += start
        txs = []
        for i in range(begin, chunk_end):
            tx = {
                "to": gen_account(cfg.global_seq, i).address,
                # 50 CRO/account: headroom over the worst-case batch configs'
                # 200 txs x 21000 gas x 5e12 gas_price = 21 CRO in ante-handler
                # fees per account - too little here means every batched
                # MsgEthereumTx past the point funds run out silently fails
                # CheckTx (insufficient funds), and the benchmark reports a
                # no_load_period with no error surfaced anywhere.
                "value": 50 * 10**18,
                "nonce": nonce,
                "gas": 21000,
                "gasPrice": cfg.gas_price,
                "chainId": cfg.chain_id,
            }
            txs.append(
                EthTx(
                    tx, fund_account.sign_transaction(tx).rawTransaction, fund_address
                )
            )
            nonce += 1

        if fund_mode == "eth":
            for tx in txs:
                w3.eth.send_raw_transaction(tx.raw)
        else:
            raw = build_cosmos_tx(
                *txs, msg_version=cfg.msg_version, evm_denom=cfg.evm_denom
            )
            rsp = requests.post(
                cfg.primary.rpc,
                json=json_rpc_send_body(raw, method="broadcast_tx_sync"),
            ).json()
            result = rsp.get("result") or {}
            if result.get("code", 0) != 0:
                raise click.ClickException(
                    f"funding broadcast failed for accounts [{begin}, {chunk_end}): "
                    f"{result.get('log')}"
                )

        # wait for nonce to change
        while w3.eth.get_transaction_count(fund_account.address) < nonce:
            time.sleep(1)

        print("sent", begin, chunk_end)


@backoff.on_exception(
    backoff.expo, ValueError, max_time=10, giveup=lambda e: "failed to load state" not in str(e)
)
def _query_account(w3, addr):
    nonce = w3.eth.get_transaction_count(addr)
    balance = int(w3.eth.get_balance(addr))
    return nonce, balance


@cli.command()
@click.option("--config", "config_path", required=True)
@click.argument("start", type=int)
@click.argument("end", type=int)
def check(config_path, start, end):
    """Query nonce/balance for generated test accounts [start, end]."""
    cfg = load_config(config_path)
    start, end = physical_account_range(start, end, cfg.num_txs, cfg.sender_strategy)
    json_rpcs = itertools.cycle(cfg.json_rpcs)
    for i in range(start, end + 1):
        w3 = web3.Web3(web3.HTTPProvider(next(json_rpcs)))
        addr = gen_account(cfg.global_seq, i).address
        # "latest" can resolve to a height the app hasn't committed yet on a
        # fast-committing chain (20ms timeout_commit) - retry that race instead
        # of aborting the whole account sweep.
        nonce, balance = _query_account(w3, addr)
        print(i, addr, nonce, balance)


@cli.command("gen-txs")
@click.option("--config", "config_path", required=True)
@click.option("--nonce", default=0)
@click.option("--start-account", default=0)
@click.option("-o", "--output", "output_path", default=None, help="default: stdout")
@click.argument("start", type=int)
@click.argument("end", type=int)
def gen_txs(config_path, nonce, start_account, output_path, start, end):
    """Generate a signed tx batch for accounts [start, end]."""
    cfg = load_config(config_path)
    num_accounts = end - start + 1
    txs = gen(
        cfg.global_seq,
        num_accounts,
        cfg.num_txs,
        cfg.tx_type,
        cfg.batch_size,
        start_account=start + start_account,
        nonce=nonce,
        msg_version=cfg.msg_version,
        tx_options=_tx_options(cfg),
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
        sender_strategy=cfg.sender_strategy,
    )
    print(
        f"generated {num_accounts * cfg.num_txs} EVM txs "
        f"in {len(txs)} {cfg.mode} txs",
        file=sys.stderr,
    )
    payload = {"num_accounts": num_accounts, "txs": txs}
    if output_path:
        Path(output_path).write_text(ujson.dumps(payload))
    else:
        ujson.dump(payload, sys.stdout)


@cli.command("send-txs")
@click.option("--config", "config_path", required=True)
@click.option("--sync/--async", default=False)
@click.argument("path", type=str)
def send_txs(config_path, sync, path):
    """Round-robin broadcast a tx batch file across all configured endpoints."""
    cfg = load_config(config_path)
    payload = ujson.loads(Path(path).read_text())
    txs = payload["txs"]
    num_accounts = payload["num_accounts"]
    rpcs = cfg.json_rpcs if cfg.mode == "eth" else cfg.rpcs
    asyncio.run(
        send_round_robin(
            txs,
            rpcs,
            sync=sync,
            batch_size=cfg.send_batch_size,
            batch_interval=cfg.send_interval,
            mode=cfg.mode,
            num_accounts=num_accounts,
        )
    )


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--count", default=30)
def stats(config_path, count):
    """Dump block/TPS/gas stats from the primary endpoint."""
    cfg = load_config(config_path)
    if cfg.mode == "eth":
        current = eth_block_number(cfg.primary.json_rpc)
        dump_eth_block_stats(
            sys.stdout,
            json_rpc=cfg.primary.json_rpc,
            start=max(2, current - count),
            end=current,
        )
        return

    current = block_height(cfg.primary.rpc)
    dump_block_stats(
        sys.stdout,
        rpc=cfg.primary.rpc,
        json_rpc=cfg.primary.json_rpc,
        telemetry=cfg.telemetry,
        start=max(2, current - count),
        end=current,
    )


def _run_bench_once(cfg, nonce, probe_batches, start, end, capture_stats, txs_cache=None):
    """Generate load for accounts [start, end] and report stats for one run.

    Returns a dict with mode, load_start, load_end, committed_txs,
    expected_txs, summary, and stats_text (None unless capture_stats).
    """
    num_accounts = end - start + 1

    cached_payload = None
    if txs_cache and Path(txs_cache).exists():
        cached_payload = ujson.loads(Path(txs_cache).read_text())
        if (
            cached_payload["num_accounts"] != num_accounts
            or cached_payload["num_txs"] != cfg.num_txs
        ):
            raise click.ClickException(
                f"--txs-cache {txs_cache} was generated for "
                f"{cached_payload['num_accounts']} accounts x {cached_payload['num_txs']} txs, "
                f"but this run covers {num_accounts} accounts x {cfg.num_txs} txs; remove the "
                "stale cache file or point --txs-cache elsewhere"
            )

    if cached_payload is not None:
        txs = cached_payload["txs"]
        print(f"loaded {len(txs)} cached {cfg.mode} txs from {txs_cache}", file=sys.stderr)
    else:
        if nonce is None:
            nonce = current_sender_nonce(cfg, start, end)
            print(f"using current sender nonce {nonce}", file=sys.stderr)

        print("generating txs...", file=sys.stderr)
        txs = gen(
            cfg.global_seq,
            num_accounts,
            cfg.num_txs,
            cfg.tx_type,
            cfg.batch_size,
            start_account=start,
            nonce=nonce,
            msg_version=cfg.msg_version,
            tx_options=_tx_options(cfg),
            evm_denom=cfg.evm_denom,
            wire_format=cfg.mode,
            sender_strategy=cfg.sender_strategy,
        )
        print(
            f"generated {num_accounts * cfg.num_txs} EVM txs "
            f"in {len(txs)} {cfg.mode} txs",
            file=sys.stderr,
        )
        if txs_cache:
            txs_cache_path = Path(txs_cache)
            txs_cache_path.parent.mkdir(parents=True, exist_ok=True)
            # write-then-rename so a crash mid-write, or a concurrent run sharing
            # this cache key, never leaves a truncated file for a reader to load.
            tmp_path = txs_cache_path.with_suffix(f"{txs_cache_path.suffix}.tmp.{os.getpid()}")
            tmp_path.write_text(
                ujson.dumps({"num_accounts": num_accounts, "num_txs": cfg.num_txs, "txs": txs})
            )
            tmp_path.replace(txs_cache_path)
            print(f"wrote tx cache to {txs_cache}", file=sys.stderr)

    stats_buffer = io.StringIO() if capture_stats else None
    stats_out = Tee(sys.stdout, stats_buffer) if capture_stats else sys.stdout

    if cfg.mode == "eth":
        load_start = eth_block_number(cfg.primary.json_rpc)
        print("sending txs...", file=sys.stderr)
        asyncio.run(
            send_round_robin(
                txs,
                cfg.json_rpcs,
                batch_size=cfg.send_batch_size,
                batch_interval=cfg.send_interval,
                mode=cfg.mode,
                num_accounts=num_accounts,
                probe_batches=probe_batches,
            )
        )
        load_end = eth_block_number(cfg.primary.json_rpc)
        load_end, committed_txs = wait_for_committed_eth_txs(
            cfg.primary.json_rpc, load_start, load_end, len(txs)
        )
        summary = dump_eth_block_stats(
            stats_out,
            json_rpc=cfg.primary.json_rpc,
            start=load_start,
            end=load_end,
        )
        print(f"committed_eth_txs {committed_txs}/{len(txs)}")
    else:
        mempool_monitor = MempoolMonitor(cfg.primary.rpc)
        stm_monitor = BlockSTMMonitor(cfg.primary.rpc, cfg.telemetry)
        prom_baseline_text = _fetch_prometheus(cfg.telemetry)
        consensus_baseline = scrape_consensus_raw(prom_baseline_text)
        consensus_health_baseline = scrape_consensus_health_raw(prom_baseline_text)
        disk_net_baseline = scrape_disk_net_raw(fetch_node_exporter(cfg.primary.node_exporter))
        if cfg.telemetry and consensus_health_baseline is None:
            print(
                "warning: telemetry baseline scrape returned no consensus-health "
                "counters; those numbers will be node-lifetime totals",
                file=sys.stderr,
            )
        if cfg.primary.node_exporter and disk_net_baseline is None:
            print(
                "warning: node_exporter baseline scrape returned no counters; "
                "disk/net numbers will be node-lifetime totals",
                file=sys.stderr,
            )

        load_start = block_height(cfg.primary.rpc)
        mempool_monitor.start()
        stm_monitor.start()
        committed_txs = 0
        try:
            print("sending txs...", file=sys.stderr)
            asyncio.run(
                send_round_robin(
                    txs,
                    cfg.rpcs,
                    batch_size=cfg.send_batch_size,
                    batch_interval=cfg.send_interval,
                    num_accounts=num_accounts,
                    probe_batches=probe_batches,
                )
            )
            load_end = block_height(cfg.primary.rpc)
            load_end, committed_txs = wait_for_committed_txs(
                cfg.primary.rpc, load_start, load_end, len(txs)
            )
        finally:
            mempool_monitor.stop()
            stm_monitor.stop()

        summary = dump_block_stats(
            stats_out,
            rpc=cfg.primary.rpc,
            json_rpc=cfg.primary.json_rpc,
            telemetry=cfg.telemetry,
            start=load_start,
            end=load_end,
            mempool_data=mempool_monitor.data,
            stm_data=stm_monitor.data,
            consensus_baseline=consensus_baseline,
            consensus_health_baseline=consensus_health_baseline,
            node_exporter=cfg.primary.node_exporter,
            disk_net_baseline=disk_net_baseline,
        )
        print(f"committed_cosmos_txs {committed_txs}/{len(txs)}")

    return {
        "mode": cfg.mode,
        "load_start": load_start,
        "load_end": load_end,
        "committed_txs": committed_txs,
        "expected_txs": len(txs),
        "summary": summary,
        "stats_text": stats_buffer.getvalue() if capture_stats else None,
    }


def _run_record_path(results_path, run_index, total_runs):
    if total_runs == 1:
        return results_path
    path = Path(results_path)
    return str(path.with_name(f"{path.stem}-run{run_index + 1}{path.suffix}"))


def _raise_on_divergence(failures):
    """State divergence is a correctness failure, not a tuning signal, so it
    fails the command unconditionally — unlike the saturation gates it is never
    something the operator opts into."""
    if failures:
        raise click.ClickException("state divergence detected: " + " | ".join(failures))


def _warn_unverified(warnings, label="divergence check unverified"):
    """A signal that can legitimately occur on a healthy network — an
    unreachable or slow node, a validator missing one precommit — establishes no
    defect, so it is surfaced and not raised."""
    for warning in warnings:
        click.echo(f"warning: {label}: {warning}", err=True)


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--nonce", type=click.IntRange(min=0), default=None)
@click.option(
    "--probe-batches",
    default=1,
    help=(
        "Send this many leading batches synchronously so CheckTx rejections "
        "surface immediately, instead of the silent no-op you get from "
        "broadcast_tx_async when every tx is rejected. Set to 0 to disable."
    ),
)
@click.option(
    "--results",
    "results_path",
    default=None,
    help=(
        "Write a run-record JSON (config snapshot, node fingerprint, "
        "per-block series, summary metrics, saturation verdict) to this path. "
        "With --repeat > 1, per-run records go to <stem>-runN<suffix> and the "
        "aggregate record goes to this path."
    ),
)
@click.option(
    "--require-saturation",
    is_flag=True,
    default=False,
    help=(
        "Exit non-zero with the failing reasons when the tuning-guide "
        "saturation gates (gas utilization, mempool pending, failed tx rate) "
        "are not met."
    ),
)
@click.option(
    "--repeat",
    default=1,
    type=click.IntRange(min=1),
    help="Run the same load N times and aggregate metrics across runs.",
)
@click.option(
    "--txs-cache",
    "txs_cache",
    default=None,
    help=(
        "Load the signed tx batch from this file instead of generating it, "
        "or write it here if the file doesn't exist yet. Only valid when the "
        "target accounts start at nonce 0 on every run (e.g. genesis-funded "
        "accounts on a freshly initialized devnet) - a stale cache replayed "
        "against accounts with a different nonce fails CheckTx."
    ),
)
@click.argument("start", type=int)
@click.argument("end", type=int)
def bench(
    config_path,
    nonce,
    probe_batches,
    results_path,
    require_saturation,
    repeat,
    txs_cache,
    start,
    end,
):
    """Generate load, send it round-robin across all endpoints, then report stats."""
    cfg = load_config(config_path)
    capture_stats = bool(results_path)

    runs = []
    for i in range(repeat):
        if repeat > 1:
            print(f"=== run {i + 1}/{repeat} ===", file=sys.stderr)
        run_nonce = nonce if i == 0 else None
        # A cached batch is only valid for the accounts' starting nonce it was
        # signed against; repeat runs beyond the first reuse the same accounts
        # at a later nonce, so they must fall back to generating fresh txs.
        run_txs_cache = txs_cache if i == 0 else None
        run = _run_bench_once(
            cfg, run_nonce, probe_batches, start, end, capture_stats, run_txs_cache
        )
        runs.append(run)
        # Sampled right after the load, while the nodes are still at the tip the
        # run drove them to.
        run["divergence"] = check_divergence(cfg.endpoints)

        if results_path:
            record = build_run_record(
                cfg=cfg,
                config_path=config_path,
                mode=run["mode"],
                load_start=run["load_start"],
                load_end=run["load_end"],
                stats_text=run["stats_text"],
                summary=run["summary"],
                committed_txs=run["committed_txs"],
                expected_txs=run["expected_txs"],
                divergence=run["divergence"],
                extra={"run_index": i} if repeat > 1 else None,
            )
            run_path = _run_record_path(results_path, i, repeat)
            write_run_record(record, run_path)
            print(f"wrote run record to {run_path}", file=sys.stderr)

    if repeat > 1 and results_path:
        aggregate = build_aggregate_record(
            cfg=cfg,
            config_path=config_path,
            summaries=[run["summary"] for run in runs],
            divergences=[run["divergence"] for run in runs],
        )
        write_run_record(aggregate, results_path)
        print(f"wrote aggregate record to {results_path}", file=sys.stderr)

    _warn_unverified(
        [
            f"run {i + 1}: {warning}"
            for i, run in enumerate(runs)
            for warning in divergence_warnings(run["divergence"])
        ]
    )
    _warn_unverified(
        [
            f"run {i + 1}: {warning}"
            for i, run in enumerate(runs)
            for warning in consensus_health_warnings(run["summary"])
        ],
        label="consensus health",
    )
    _raise_on_divergence(
        [
            f"run {i + 1}: {reason}"
            for i, run in enumerate(runs)
            for reason in divergence_reasons(run["divergence"])
            + consensus_health_reasons(run["summary"])
        ]
    )

    if require_saturation:
        failing = []
        for i, run in enumerate(runs):
            ok, reasons = evaluate_saturation(run["summary"])
            if not ok:
                failing.append(f"run {i + 1}: " + "; ".join(reasons))
        if failing:
            raise click.ClickException(
                "saturation gates not met: " + " | ".join(failing)
            )

    uncommitted = [run for run in runs if run["committed_txs"] < run["expected_txs"]]
    if uncommitted:
        kind = "Ethereum" if cfg.mode == "eth" else "Cosmos"
        if repeat == 1:
            run = uncommitted[0]
            raise click.ClickException(
                f"timed out waiting for generated transactions to commit: "
                f"{run['committed_txs']}/{run['expected_txs']} {kind} transactions committed"
            )
        details = "; ".join(
            f"run {runs.index(run) + 1}: {run['committed_txs']}/{run['expected_txs']}"
            for run in uncommitted
        )
        raise click.ClickException(
            f"timed out waiting for generated transactions to commit "
            f"({kind}): {details}"
        )


def _soak_batch_size(rate, batch_interval, evm_txs_per_wire_tx, warn=True):
    """Wire txs to send per batch to sustain `rate` EVM tx/s.

    `evm_txs_per_wire_tx` is the effective packing, not the configured
    batch_size: `gen` batches only within one account, so it is
    min(num_txs_per_account, batch_size). A single wire tx per batch is already
    the floor rate. Targets below that floor can only be met by overshooting,
    which would silently benchmark a different rate than the operator asked
    for.
    """
    per_wire_tx = max(1, evm_txs_per_wire_tx)
    min_rate = per_wire_tx / batch_interval
    if rate < min_rate:
        raise click.ClickException(
            f"target rate {rate:g} tx/s is below the {min_rate:g} tx/s floor set by "
            f"batch_size={per_wire_tx}: lower batch_size or raise --rate"
        )
    exact = rate * batch_interval / per_wire_tx
    batch_size = round(exact)
    # Achievable, but only at a quantised rate: warn rather than reject so the
    # operator knows which rate the numbers they get actually describe.
    if warn and abs(batch_size - exact) > 0.05 * exact:
        effective_rate = batch_size * per_wire_tx / batch_interval
        click.echo(
            f"warning: target rate {rate:g} tx/s is not reachable in whole wire txs "
            f"with batch_size={per_wire_tx}; using {effective_rate:g} tx/s",
            err=True,
        )
    return batch_size


# Each pass raises num_txs, which raises the packing and so lowers the batch size
# the next pass needs; a handful of passes is far more than the crossing takes.
_SOAK_SIZING_PASSES = 8


def _soak_tx_supply(rate, duration, num_accounts, batch_interval, cfg_batch_size):
    """(num_txs per account, wire txs per batch) sized so the paced sender cannot
    run out of txs before `duration` elapses.

    Pacing rounds to a whole number of wire txs per batch, so the rate actually
    sent can overshoot the requested one. Sizing the supply from the requested
    rate then drains it early: the sender returns, the checkpoint sampler is
    stopped before the final interval closes, and a healthy soak fails for want
    of a second checkpoint. Coverage has to be counted in wire txs, since that is
    what the sender consumes.

    Packing is min(num_txs, cfg.batch_size), so the batch size depends on the
    supply that depends on the batch size; this iterates until the supply covers
    the batch size it implies.
    """
    num_txs = max(1, math.ceil(rate * duration / num_accounts))
    for _ in range(_SOAK_SIZING_PASSES):
        per_wire_tx = max(1, min(num_txs, cfg_batch_size))
        batch_size = _soak_batch_size(rate, batch_interval, per_wire_tx, warn=False)
        wire_txs_needed = math.ceil(batch_size * duration / batch_interval)
        wire_txs_generated = num_accounts * math.ceil(num_txs / per_wire_tx)
        if wire_txs_generated >= wire_txs_needed:
            break
        num_txs = per_wire_tx * math.ceil(wire_txs_needed / num_accounts)
    per_wire_tx = max(1, min(num_txs, cfg_batch_size))
    return num_txs, _soak_batch_size(rate, batch_interval, per_wire_tx)


def _wait_out_soak_duration(started, duration):
    """Hold until `duration` has elapsed since `started`.

    The sampler emits a checkpoint at the end of each interval, so returning as
    soon as the sender drains would drop the final one.
    """
    remaining = duration - (time.monotonic() - started)
    if remaining > 0:
        time.sleep(remaining)


def _check_soak_duration(duration, checkpoint_interval):
    """Trend fitting needs two checkpoints, and the sampler emits one per
    interval, so a too-short soak can only fail after burning the whole run.
    """
    if duration < 2 * checkpoint_interval:
        raise click.ClickException(
            f"--duration {duration:g}s yields fewer than 2 checkpoints at "
            f"--checkpoint-interval {checkpoint_interval:g}s, so no trend can be "
            f"fitted: raise --duration to at least {2 * checkpoint_interval:g}s or "
            "lower --checkpoint-interval"
        )


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--nonce", type=click.IntRange(min=0), default=None)
@click.option("--rate", type=float, required=True, help="target tx/s")
@click.option("--duration", type=float, required=True, help="soak duration in seconds")
@click.option("--checkpoint-interval", type=float, default=30.0)
@click.option(
    "--results",
    "results_path",
    default=None,
    help="write soak checkpoints/trends/verdict as JSON to this path",
)
@click.argument("start", type=int)
@click.argument("end", type=int)
def soak(config_path, nonce, rate, duration, checkpoint_interval, results_path, start, end):
    """Open-loop soak: pace load at --rate tx/s for --duration seconds,
    sampling periodic checkpoints and trend-fitting RSS/TPS/block time
    for a leak/degradation verdict."""
    cfg = load_config(config_path)
    if cfg.mode == "eth":
        raise click.ClickException("soak currently only supports cosmos mode")
    _check_soak_duration(duration, checkpoint_interval)

    num_accounts = end - start + 1
    # A batch every second, sized to hit the target rate, paces sends across the
    # soak duration without waiting for prior batches to commit.
    batch_interval = 1.0
    num_txs, batch_size = _soak_tx_supply(
        rate, duration, num_accounts, batch_interval, cfg.batch_size
    )
    # Nonces are checked after num_txs is known: it selects the physical sender
    # range under unique-per-tx, so the check has to cover exactly the accounts
    # gen() below signs from.
    if nonce is None:
        nonce = current_sender_nonce(cfg, start, end, num_txs=num_txs)
        print(f"using current sender nonce {nonce}", file=sys.stderr)

    print(
        f"generating ~{num_txs} txs/account for a {duration:.0f}s soak at {rate:.1f} tx/s...",
        file=sys.stderr,
    )
    txs = gen(
        cfg.global_seq,
        num_accounts,
        num_txs,
        cfg.tx_type,
        cfg.batch_size,
        start_account=start,
        nonce=nonce,
        msg_version=cfg.msg_version,
        tx_options=_tx_options(cfg),
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
        sender_strategy=cfg.sender_strategy,
    )

    sampler = CheckpointSampler(cfg.primary.rpc, cfg.telemetry, checkpoint_interval)
    started = time.monotonic()
    sampler.start()
    try:
        print("sending txs...", file=sys.stderr)
        asyncio.run(
            send_round_robin(
                txs,
                cfg.rpcs,
                batch_size=batch_size,
                batch_interval=batch_interval,
                num_accounts=num_accounts,
                deadline_s=duration,
            )
        )
        _wait_out_soak_duration(started, duration)
    finally:
        sampler.stop()

    checkpoints = sampler.checkpoints
    print()
    print("=== Soak Checkpoints ===")
    for c in checkpoints:
        block_time = f"{c['avg_block_time_ms']:.0f}ms" if c["avg_block_time_ms"] is not None else "N/A"
        rss = f"{c['rss_bytes']:.0f}" if c["rss_bytes"] is not None else "N/A"
        tps = f"{c['tps']:.2f}" if c["tps"] is not None else "N/A"
        print(
            f"t={c['elapsed_s']:.0f}s height={c['height']} tps={tps}"
            f" block_time={block_time} rss_bytes={rss}"
        )

    trends = fit_trends(checkpoints)
    verdict = soak_verdict(trends, checkpoints, cfg.telemetry)
    divergence = check_divergence(cfg.endpoints)
    print()
    print("=== Soak Verdict ===")
    for key, slope in trends.items():
        print(f"{key}_trend_per_s {slope if slope is not None else 'N/A'}")
    for gate, state in verdict["gates"].items():
        print(f"gate {gate}: {state}")
    print(f"ok {verdict['ok']}")
    for reason in verdict["reasons"]:
        print(f"  {reason}")
    for reason in divergence_reasons(divergence):
        print(f"  divergence: {reason}")
    for warning in divergence_warnings(divergence):
        print(f"  divergence unverified: {warning}")

    if results_path:
        Path(results_path).write_text(
            ujson.dumps(
                {
                    "checkpoints": checkpoints,
                    "trends": trends,
                    "verdict": verdict,
                    "divergence": divergence,
                },
                indent=2,
                default=str,
            )
        )
        print(f"wrote soak record to {results_path}", file=sys.stderr)

    _raise_on_divergence(divergence_reasons(divergence))
    if not verdict["ok"]:
        raise click.ClickException("soak flagged: " + "; ".join(verdict["reasons"]))


@cli.command("sweep")
@click.option("--config", "config_path", required=True)
@click.option("--nonce", type=click.IntRange(min=0), default=None)
@click.option(
    "--results-dir",
    "results_dir",
    required=True,
    help="directory to write one run record per cell plus sweep-summary.txt",
)
@click.option(
    "--no-stop-on-degradation",
    "stop_on_degradation",
    is_flag=True,
    default=True,
    flag_value=False,
    help="keep running every cell even after one fails the saturation gates",
)
@click.argument("matrix_path", type=str)
@click.argument("start", type=int)
@click.argument("end", type=int)
def sweep_cmd(config_path, nonce, results_dir, stop_on_degradation, matrix_path, start, end):
    """Sweep a parameter matrix: apply-config hook + bench per cell.

    MATRIX_PATH is a JSON/YAML file: {apply_config_hook, restart_wait_s,
    axes: {name: [values, ...]}}. Stops at the first cell that fails the
    saturation gates unless --no-stop-on-degradation is passed.
    """
    cfg = load_config(config_path)
    matrix = load_matrix(_load_json_or_yaml(matrix_path))

    results_path = Path(results_dir)
    results_path.mkdir(parents=True, exist_ok=True)

    # Only the first cell gets the explicit nonce; every later cell passes
    # None so _run_bench_once re-queries the live chain nonce, since earlier
    # cells already consumed nonces by sending transactions.
    cell_index = 0
    divergence_failures = []
    divergence_unverified = []
    health_warnings = []
    undercommitted = []

    def run_cell(cell):
        nonlocal cell_index
        run_nonce = nonce if cell_index == 0 else None
        cell_index += 1
        run = _run_bench_once(cfg, run_nonce, 1, start, end, capture_stats=True)
        divergence = check_divergence(cfg.endpoints)
        record = build_run_record(
            cfg=cfg,
            config_path=config_path,
            mode=run["mode"],
            load_start=run["load_start"],
            load_end=run["load_end"],
            stats_text=run["stats_text"],
            summary=run["summary"],
            committed_txs=run["committed_txs"],
            expected_txs=run["expected_txs"],
            run_kind="sweep-cell",
            divergence=divergence,
            extra={"cell": cell},
        )
        # Safe to interpolate into a path: cell keys/values come from the
        # operator's own local sweep matrix file, same trust assumption as
        # sweep.apply_config's shell hook.
        cell_name = "-".join(f"{k}{v}" for k, v in cell.items()) or "cell"
        write_run_record(record, results_path / f"{cell_name}.json")
        divergence_failures.extend(
            f"{cell_name}: {reason}"
            for reason in divergence_reasons(divergence)
            + consensus_health_reasons(run["summary"])
        )
        divergence_unverified.extend(
            f"{cell_name}: {warning}"
            for warning in divergence_warnings(divergence)
        )
        health_warnings.extend(
            f"{cell_name}: {warning}"
            for warning in consensus_health_warnings(run["summary"])
        )
        if run["committed_txs"] < run["expected_txs"]:
            undercommitted.append(
                f"{cell_name}: {run['committed_txs']}/{run['expected_txs']}"
            )
        return run["summary"]

    print(f"sweeping {len(matrix['cells'])} cells...", file=sys.stderr)
    cell_results = run_sweep(matrix, run_cell, stop_on_degradation=stop_on_degradation)

    report = summarize_sweep(cell_results)
    print(report)
    (results_path / "sweep-summary.txt").write_text(report + "\n")

    ran = len(cell_results)
    total = len(matrix["cells"])
    if ran < total:
        print(
            f"stopped after {ran}/{total} cells: cell {ran} failed the saturation gates",
            file=sys.stderr,
        )

    _warn_unverified(divergence_unverified)
    _warn_unverified(health_warnings, label="consensus health")
    _raise_on_divergence(divergence_failures)
    # Same hard failure `bench` applies: a cell whose load never fully committed
    # measured a truncated window, so its numbers describe a different run than
    # the one requested.
    if undercommitted:
        kind = "Ethereum" if cfg.mode == "eth" else "Cosmos"
        raise click.ClickException(
            f"timed out waiting for generated transactions to commit "
            f"({kind}): " + "; ".join(undercommitted)
        )
    # A sweep whose every cell (or only its last cell) failed the gates has to
    # exit non-zero: `ran == total` alone says nothing about the verdicts.
    failed = [entry for entry in cell_results if not entry["ok"]]
    if failed:
        details = " | ".join(
            (" ".join(f"{k}={v}" for k, v in entry["cell"].items()) or "(no params)")
            + ": "
            + "; ".join(entry["reasons"])
            for entry in failed
        )
        raise click.ClickException(
            f"saturation gates not met in {len(failed)}/{ran} run cells: {details}"
        )


@cli.command()
@click.option("-o", "--output", "output_path", default=None, help="write HTML report here")
@click.argument("record_a_path", metavar="A.json", type=str)
@click.argument("record_b_path", metavar="B.json", type=str)
def compare(output_path, record_a_path, record_b_path):
    """Compare two bench run records: delta table, config diff, optional HTML."""
    record_a = load_record(record_a_path)
    record_b = load_record(record_b_path)
    comparison = build_comparison(record_a, record_b, record_a_path, record_b_path)
    print(render_comparison_text(comparison))
    if output_path:
        write_comparison_html(comparison, output_path)
        print(f"wrote comparison report to {output_path}", file=sys.stderr)


def _load_json_or_yaml(path):
    text = Path(path).read_text()
    return ujson.loads(text) if path.endswith(".json") else yaml.safe_load(text)


@cli.command("bootstrap-peers")
@click.option("--port", default=26656)
@click.option("-o", "--output", "output_path", default=None, help="default: stdout")
@click.argument("nodes_path", type=str)
def bootstrap_peers_cmd(port, output_path, nodes_path):
    """Derive libp2p peer IDs and generate bootstrap_peers for N nodes.

    NODES_PATH is a JSON/YAML file: [{name, ip, node_key_path}, ...].
    """
    nodes = _load_json_or_yaml(nodes_path)
    payload = ujson.dumps(bootstrap_peers(nodes, port=port), indent=2)
    if output_path:
        Path(output_path).write_text(payload)
    else:
        print(payload)


@cli.command()
@click.option("--config", "config_path", required=True)
def preflight(config_path):
    """RPC-only devnet preflight: resolved mempool type + peer connectivity matrix.

    Sysctl tuning and the libp2p-transport-enabled log line need host
    access, which this tool doesn't have; check those manually.
    """
    cfg = load_config(config_path)
    failures = []

    print("mempool.type by node:")
    mempool_types = resolved_mempool_types(cfg.endpoints)
    for name, mtype in mempool_types.items():
        print(f"  {name}: {mtype or '(undeclared)'}")
    declared = {v for v in mempool_types.values() if v}
    # Undeclared everywhere just means nobody told the tool; two *different*
    # declared types mean the nodes are running different mempools, and any
    # number measured across them describes neither configuration.
    if len(declared) > 1:
        failures.append(f"nodes disagree on mempool.type: {mempool_types}")

    print("peer connectivity matrix:")
    if len(cfg.endpoints) < 2:
        print("  (single endpoint: no peer links to verify)")
    probe = probe_peers(cfg.endpoints)
    matrix = peer_connectivity_matrix(cfg.endpoints, probe=probe)
    for name, row in matrix.items():
        print(f"  {name}: " + ", ".join(f"{other}={v}" for other, v in row.items()))

    unreachable = unreachable_nodes(*probe)
    missing_links = [
        f"{name}->{other}"
        for name, row in matrix.items()
        for other, v in row.items()
        if v is False
    ]
    if unreachable:
        failures.append(f"unreachable nodes: {unreachable}")
    if missing_links:
        failures.append(f"missing peer links: {missing_links}")
    if failures:
        raise click.ClickException("preflight failed: " + "; ".join(failures))


if __name__ == "__main__":
    cli()
