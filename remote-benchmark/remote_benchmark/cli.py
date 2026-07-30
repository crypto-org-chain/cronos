import asyncio
import io
import itertools
import sys
import time
from pathlib import Path

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
from .preflight import peer_connectivity_matrix, resolved_mempool_types
from .results import (
    build_aggregate_record,
    build_run_record,
    evaluate_saturation,
    write_run_record,
)
from .resources import fetch_node_exporter, scrape_disk_net_raw
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


def wait_for_committed_txs(rpc, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT):
    """Extend the sample until all generated Cosmos txs are committed."""
    # ``start`` is the pre-send anchor and can still contain setup traffic,
    # so only count envelopes committed after it.
    next_height = start + 1
    committed_txs = 0
    deadline = time.monotonic() + timeout

    while True:
        while next_height <= end:
            committed_txs += len(block_txs(next_height, rpc) or [])
            next_height += 1
            if committed_txs >= expected_txs:
                return end, committed_txs

        if time.monotonic() >= deadline:
            return end, committed_txs

        current = block_height(rpc)
        if current > end:
            end = current
        else:
            time.sleep(0.2)


def wait_for_committed_eth_txs(
    json_rpc, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT
):
    """Extend the sample until all generated Ethereum txs are committed."""
    next_height = start + 1
    committed_txs = 0
    deadline = time.monotonic() + timeout

    while True:
        while next_height <= end:
            committed_txs += len(block_eth(next_height, json_rpc)["transactions"])
            next_height += 1
            if committed_txs >= expected_txs:
                return end, committed_txs

        if time.monotonic() >= deadline:
            return end, committed_txs

        current = eth_block_number(json_rpc)
        if current > end:
            end = current
        else:
            time.sleep(0.2)


def current_sender_nonce(cfg, start, end):
    """Return the shared current nonce for the benchmark's physical senders."""
    physical_start, physical_end = physical_account_range(
        start, end, cfg.num_txs, cfg.sender_strategy
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
            if rsp["result"]["code"] != 0:
                print(rsp["result"]["log"])
                break

        # wait for nonce to change
        while w3.eth.get_transaction_count(fund_account.address) < nonce:
            time.sleep(1)

        print("sent", begin, chunk_end)


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
        nonce = w3.eth.get_transaction_count(addr)
        balance = int(w3.eth.get_balance(addr))
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
        tx_options={"gas_price": cfg.gas_price, "chain_id": cfg.chain_id},
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


def _run_bench_once(cfg, nonce, probe_batches, start, end, capture_stats):
    """Generate load for accounts [start, end] and report stats for one run.

    Returns a dict with mode, load_start, load_end, committed_txs,
    expected_txs, summary, and stats_text (None unless capture_stats).
    """
    num_accounts = end - start + 1
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
        tx_options={"gas_price": cfg.gas_price, "chain_id": cfg.chain_id},
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
        sender_strategy=cfg.sender_strategy,
    )
    print(
        f"generated {num_accounts * cfg.num_txs} EVM txs "
        f"in {len(txs)} {cfg.mode} txs",
        file=sys.stderr,
    )

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
@click.argument("start", type=int)
@click.argument("end", type=int)
def bench(
    config_path,
    nonce,
    probe_batches,
    results_path,
    require_saturation,
    repeat,
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
        run = _run_bench_once(cfg, run_nonce, probe_batches, start, end, capture_stats)
        runs.append(run)

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
        )
        write_run_record(aggregate, results_path)
        print(f"wrote aggregate record to {results_path}", file=sys.stderr)

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


def _load_nodes(path):
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
    nodes = _load_nodes(nodes_path)
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

    print("mempool.type by node:")
    mempool_types = resolved_mempool_types(cfg.endpoints)
    for name, mtype in mempool_types.items():
        print(f"  {name}: {mtype or '(undeclared)'}")
    declared = {v for v in mempool_types.values() if v}
    if len(declared) > 1:
        print(f"  WARNING: nodes disagree on mempool.type: {declared}", file=sys.stderr)

    print("peer connectivity matrix:")
    matrix = peer_connectivity_matrix(cfg.endpoints)
    for name, row in matrix.items():
        print(f"  {name}: " + ", ".join(f"{other}={v}" for other, v in row.items()))

    unreachable = [name for name, row in matrix.items() if row and all(v is None for v in row.values())]
    missing_links = [
        f"{name}->{other}"
        for name, row in matrix.items()
        for other, v in row.items()
        if v is False
    ]
    if unreachable or missing_links:
        raise click.ClickException(
            "preflight failed: "
            + (f"unreachable nodes: {unreachable}; " if unreachable else "")
            + (f"missing peer links: {missing_links}" if missing_links else "")
        )


if __name__ == "__main__":
    cli()
