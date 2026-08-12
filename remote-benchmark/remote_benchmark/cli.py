import asyncio
import itertools
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
from .preflight import (
    peer_connectivity_matrix,
    probe_peers,
    resolved_mempool_types,
    unreachable_nodes,
)
from .results import (
    build_aggregate_record,
    build_run_record,
    evaluate_saturation,
    format_undercommitted,
    gate_run,
    write_run_record,
)
from .runner import current_sender_nonce, run_bench_once, tx_options
from .soak import (
    CheckpointSampler,
    fit_trends,
    soak_tx_supply,
    soak_verdict,
    wait_out_soak_duration,
)
from .sweep import load_matrix, run_sweep, summarize_sweep
from .stats import dump_block_stats, dump_eth_block_stats
from .transaction import (
    EthTx,
    build_cosmos_tx,
    gen,
    json_rpc_send_body,
    physical_account_range,
    send_round_robin,
)
from .utils import block_height, eth_block_number, gen_account, split_batch

# reserved for the funding account, index 0 is the funder itself.
FUND_ACCOUNT_INDEX = 0


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
    """Query nonce/balance for generated test accounts [start, end].

    Prints one line per account with an unfunded balance or nonzero nonce
    (either means the genesis-funding step didn't do its job), then a
    summary count - not every account, since a healthy run has nothing
    interesting to say about thousands of identical funded accounts.
    """
    cfg = load_config(config_path)
    start, end = physical_account_range(start, end, cfg.num_txs, cfg.sender_strategy)
    json_rpcs = itertools.cycle(cfg.json_rpcs)
    total = end - start + 1
    unfunded = 0
    last_log = time.monotonic()
    for checked, i in enumerate(range(start, end + 1), start=1):
        w3 = web3.Web3(web3.HTTPProvider(next(json_rpcs)))
        addr = gen_account(cfg.global_seq, i).address
        # "latest" can resolve to a height the app hasn't committed yet on a
        # fast-committing chain (20ms timeout_commit) - retry that race instead
        # of aborting the whole account sweep.
        nonce, balance = _query_account(w3, addr)
        if balance == 0 or nonce != 0:
            unfunded += 1
            print(i, addr, nonce, balance)
        now = time.monotonic()
        if now - last_log >= 3:
            print(f"checked {checked}/{total} accounts, {unfunded} unfunded/unexpected so far")
            last_log = now
    print(f"checked {total} accounts, {unfunded} unfunded/unexpected")


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
        tx_options=tx_options(cfg),
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
    rpcs = cfg.json_rpc_candidates if cfg.mode == "eth" else cfg.rpc_candidates
    failed = asyncio.run(
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
    if failed:
        print(f"{failed}/{len(txs)} txs never reached the mempool (send retries exhausted)", file=sys.stderr)


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
            json_rpc=cfg.primary.json_rpc_candidates,
            start=max(2, current - count),
            end=current,
        )
        return

    current = block_height(cfg.primary.rpc)
    dump_block_stats(
        sys.stdout,
        rpc=cfg.primary.rpc_candidates,
        json_rpc=cfg.primary.json_rpc_candidates,
        telemetry=cfg.telemetry,
        start=max(2, current - count),
        end=current,
    )


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
    gate_reasons = []
    gate_divergence_warnings = []
    gate_health_warnings = []
    for i in range(repeat):
        if repeat > 1:
            print(f"=== run {i + 1}/{repeat} ===", file=sys.stderr)
        run_nonce = nonce if i == 0 else None
        # A cached batch is only valid for the accounts' starting nonce it was
        # signed against; repeat runs beyond the first reuse the same accounts
        # at a later nonce, so they must fall back to generating fresh txs.
        run_txs_cache = txs_cache if i == 0 else None
        try:
            run = run_bench_once(
                cfg, run_nonce, probe_batches, start, end, capture_stats, run_txs_cache
            )
        except ValueError as e:
            raise click.ClickException(str(e)) from e
        runs.append(run)
        # gate_run samples divergence right after the load, while the nodes
        # are still at the tip the run drove them to.
        gate = gate_run(cfg, run)
        gate_reasons += [f"run {i + 1}: {reason}" for reason in gate["reasons"]]
        gate_divergence_warnings += [
            f"run {i + 1}: {warning}" for warning in gate["divergence_warnings"]
        ]
        gate_health_warnings += [
            f"run {i + 1}: {warning}" for warning in gate["health_warnings"]
        ]

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

    _warn_unverified(gate_divergence_warnings)
    _warn_unverified(gate_health_warnings, label="consensus health")
    _raise_on_divergence(gate_reasons)

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

    uncommitted = [
        (f"run {i + 1}" if repeat > 1 else None, run["committed_txs"], run["expected_txs"])
        for i, run in enumerate(runs)
        if run["committed_txs"] < run["expected_txs"]
    ]
    if uncommitted:
        kind = "Ethereum" if cfg.mode == "eth" else "Cosmos"
        raise click.ClickException(format_undercommitted(kind, uncommitted))


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
    try:
        num_txs, batch_size = soak_tx_supply(
            rate, duration, num_accounts, batch_interval, cfg.batch_size
        )
    except ValueError as e:
        raise click.ClickException(str(e)) from e
    # Nonces are checked after num_txs is known: it selects the physical sender
    # range under unique-per-tx, so the check has to cover exactly the accounts
    # gen() below signs from.
    if nonce is None:
        try:
            nonce = current_sender_nonce(cfg, start, end, num_txs=num_txs)
        except ValueError as e:
            raise click.ClickException(str(e)) from e
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
        tx_options=tx_options(cfg),
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
        sender_strategy=cfg.sender_strategy,
    )

    sampler = CheckpointSampler(cfg.primary.rpc, cfg.telemetry, checkpoint_interval)
    started = time.monotonic()
    sampler.start()
    try:
        print("sending txs...", file=sys.stderr)
        failed = asyncio.run(
            send_round_robin(
                txs,
                cfg.rpc_candidates,
                batch_size=batch_size,
                batch_interval=batch_interval,
                num_accounts=num_accounts,
                deadline_s=duration,
            )
        )
        if failed:
            print(
                f"warning: {failed}/{len(txs)} txs never reached the mempool "
                "(send retries exhausted)",
                file=sys.stderr,
            )
        wait_out_soak_duration(started, duration)
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
    soak_run = {"summary": None}
    gate = gate_run(cfg, soak_run)
    divergence = soak_run["divergence"]
    print()
    print("=== Soak Verdict ===")
    for key, slope in trends.items():
        print(f"{key}_trend_per_s {slope if slope is not None else 'N/A'}")
    for gate_key, state in verdict["gates"].items():
        print(f"gate {gate_key}: {state}")
    print(f"ok {verdict['ok']}")
    for reason in verdict["reasons"]:
        print(f"  {reason}")
    for reason in gate["reasons"]:
        print(f"  divergence: {reason}")
    for warning in gate["divergence_warnings"]:
        print(f"  divergence unverified: {warning}")
    for warning in gate["health_warnings"]:
        print(f"  consensus health unverified: {warning}")

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

    _raise_on_divergence(gate["reasons"])
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
    # None so run_bench_once re-queries the live chain nonce, since earlier
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
        try:
            run = run_bench_once(cfg, run_nonce, 1, start, end, capture_stats=True)
        except ValueError as e:
            raise click.ClickException(str(e)) from e
        gate = gate_run(cfg, run)
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
            divergence=run["divergence"],
            extra={"cell": cell},
        )
        # Safe to interpolate into a path: cell keys/values come from the
        # operator's own local sweep matrix file, same trust assumption as
        # sweep.apply_config's shell hook.
        cell_name = "-".join(f"{k}{v}" for k, v in cell.items()) or "cell"
        write_run_record(record, results_path / f"{cell_name}.json")
        divergence_failures.extend(f"{cell_name}: {reason}" for reason in gate["reasons"])
        divergence_unverified.extend(
            f"{cell_name}: {warning}" for warning in gate["divergence_warnings"]
        )
        health_warnings.extend(
            f"{cell_name}: {warning}" for warning in gate["health_warnings"]
        )
        if run["committed_txs"] < run["expected_txs"]:
            undercommitted.append(
                (cell_name, run["committed_txs"], run["expected_txs"])
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
        raise click.ClickException(format_undercommitted(kind, undercommitted))
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
