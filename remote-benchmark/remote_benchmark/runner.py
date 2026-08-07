"""Load-generation engine: generate/send a signed tx batch for [start, end]
and wait for it to commit.

Kept free of Click so it can be unit tested directly, instead of only
through CliRunner: callers (cli.py's `bench` and `sweep` commands) translate
a `ValueError` raised here into a `click.ClickException` at the CLI
boundary.
"""

import asyncio
import io
import os
import sys
import time
from pathlib import Path

import ujson
import web3

from .config import Config
from .monitor import BlockSTMMonitor, MempoolMonitor
from .resources import fetch_node_exporter, scrape_disk_net_raw
from .stats import (
    _fetch_prometheus,
    dump_block_stats,
    dump_eth_block_stats,
    scrape_consensus_health_raw,
    scrape_consensus_raw,
)
from .transaction import gen, physical_account_range, send_round_robin
from .utils import Tee, block_eth, block_height, block_txs, eth_block_number, gen_account

LOAD_COMMIT_TIMEOUT = Config.model_fields["commit_timeout"].default
PROGRESS_INTERVAL_S = 3


def tx_options(cfg) -> dict:
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
                # stop at the height that actually hit the threshold, not the
                # (possibly further-extended) outer `end` - returning `end`
                # here overshoots and lets downstream block-stats recount
                # blocks past the drain point, inflating totals past what was
                # sent.
                return next_height - 1, committed_txs

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
        raise ValueError(
            f"benchmark sender accounts have different nonces ({values}); "
            "pass --nonce explicitly or use a fresh account range"
        )
    return nonces.pop()


def _send_and_report_failures(txs, rpcs, **send_kwargs):
    """Round-robin send `txs`, warning (not raising) on any that never reached
    the mempool: the sender already retried them, and the caller still needs
    to wait out whatever did land rather than abort on a partial send.
    """
    failed = asyncio.run(send_round_robin(txs, rpcs, **send_kwargs))
    if failed:
        print(
            f"warning: {failed}/{len(txs)} txs never reached the mempool "
            "(send retries exhausted)",
            file=sys.stderr,
        )
    return failed


def run_bench_once(cfg, nonce, probe_batches, start, end, capture_stats, txs_cache=None):
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
            raise ValueError(
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
        failed = _send_and_report_failures(
            txs,
            cfg.json_rpcs,
            batch_size=cfg.send_batch_size,
            batch_interval=cfg.send_interval,
            mode=cfg.mode,
            num_accounts=num_accounts,
            probe_batches=probe_batches,
        )
        load_end = eth_block_number(cfg.primary.json_rpc)
        load_end, committed_txs = wait_for_committed_eth_txs(
            cfg.primary.json_rpc,
            load_start,
            load_end,
            len(txs) - failed,
            timeout=cfg.commit_timeout,
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
        failed = 0
        try:
            print("sending txs...", file=sys.stderr)
            failed = _send_and_report_failures(
                txs,
                cfg.rpcs,
                batch_size=cfg.send_batch_size,
                batch_interval=cfg.send_interval,
                num_accounts=num_accounts,
                probe_batches=probe_batches,
            )
            load_end = block_height(cfg.primary.rpc)
            load_end, committed_txs = wait_for_committed_txs(
                cfg.primary.rpc,
                load_start,
                load_end,
                len(txs) - failed,
                timeout=cfg.commit_timeout,
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
        "expected_txs": len(txs) - failed,
        "summary": summary,
        "stats_text": stats_buffer.getvalue() if capture_stats else None,
    }
