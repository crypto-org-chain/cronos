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
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import ujson
import web3

from .cometbft_metrics import (
    scrape_cronos_mempool_raw,
    scrape_mempool_health_raw,
    scrape_sdk_tx_metrics,
)
from .config import Config
from .monitor import BlockSTMMonitor, MempoolMonitor
from .promtext import fetch_sdk_prometheus_text
from .resources import fetch_node_exporter, scrape_disk_net_raw
from .stats import (
    _fetch_prometheus,
    dump_block_stats,
    dump_eth_block_stats,
    scrape_consensus_health_raw,
    scrape_consensus_raw,
)
from .transaction import (
    gen,
    physical_account_range,
    resend_missing_nonces,
    send_multiprocess,
    send_round_robin,
    sender_affinity_accounts,
)
from .utils import Tee, block_eth, block_height, blockchain_range, eth_block_number, gen_account

LOAD_COMMIT_TIMEOUT = Config.model_fields["commit_timeout"].default
PROGRESS_INTERVAL_S = 3
# A stuck tx (e.g. an app-mempool recheck silently dropping it, invisible to
# any client-side retry) never arrives - waiting the full timeout for it
# just burns minutes doing nothing. Bail once the commit count hasn't moved
# for this many blocks, rather than waiting out the deadline.
STALL_BLOCKS = 10
# Cap each count_txs_batch call to this many heights so a scan spanning
# thousands of blocks still checks the progress-print/timeout deadline
# between chunks, instead of blocking for the whole remaining range in one
# uninterruptible call.
WAIT_SCAN_CHUNK = 200
# eth_getBlockByNumber has no batch endpoint, so each height in the chunk is
# its own HTTP call - a chunk this small still checks the deadline often but
# caps how many heights get fetched past the point the commit threshold is
# already hit.
WAIT_SCAN_CHUNK_ETH = 20
# Retries for current_sender_nonce's post-run nonce scan: a repeat run's
# nonce check fires right after the prior run reports committed, while a
# few accounts' last txs may still be in-flight (async broadcast/retry
# lag) - short-lived divergence, not a real inconsistency.
NONCE_SETTLE_RETRIES = 5
NONCE_SETTLE_RETRY_DELAY = 2
# Nonce queries are one HTTP round trip per account with no batch endpoint -
# fan them out to bound wall time on large ranges over a high-latency link.
NONCE_QUERY_WORKERS = 64


def tx_options(cfg) -> dict:
    return {
        "gas_price": cfg.gas_price,
        "chain_id": cfg.chain_id,
        "mix": cfg.mix_weights,
    }


def gen_from_config(cfg, num_accounts, num_txs, start_account, nonce):
    """Generate signed txs with every layout/signing parameter taken from
    `cfg`, so warm-up, the measured load, `gen-txs` and `soak` cannot drift
    apart on how they build a tx."""
    return gen(
        cfg.global_seq,
        num_accounts,
        num_txs,
        cfg.tx_type,
        cfg.batch_size,
        start_account=start_account,
        nonce=nonce,
        msg_version=cfg.msg_version,
        tx_options=tx_options(cfg),
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
        sender_strategy=cfg.sender_strategy,
    )


def _wait_for_committed(
    get_height,
    count_txs_batch,
    start,
    end,
    expected_txs,
    timeout=LOAD_COMMIT_TIMEOUT,
    chunk=WAIT_SCAN_CHUNK,
    stall_blocks=STALL_BLOCKS,
    initial_committed=0,
):
    """Extend the sample until `expected_txs` have been counted committed.

    ``start`` is the pre-send anchor and can still contain setup traffic,
    so only count txs committed after it. ``count_txs_batch(lo, hi)`` returns
    a ``{height: num_txs}`` map for ``[lo, hi]``. For callers backed by a real
    batch endpoint (e.g. CometBFT's /blockchain page) a single call can cover
    many heights cheaply, so ``chunk`` can stay large. Callers with no batch
    endpoint (e.g. per-height eth_getBlockByNumber) still fetch one height per
    call internally, so a small ``chunk`` limits how many heights get fetched
    past the point where the threshold is already satisfied.

    Also gives up once the commit count hasn't moved for ``stall_blocks``
    committed blocks - a tx dropped by mempool recheck never arrives, and
    that's indistinguishable from "still catching up" except by this kind of
    stall, so without it every stuck run just burns the full ``timeout``.
    """
    next_height = start + 1
    committed_txs = initial_committed
    deadline = time.monotonic() + timeout
    started = time.monotonic()
    last_log = started
    stall_committed_txs = committed_txs
    stall_since_height = next_height - 1

    while True:
        if next_height <= end:
            chunk_end = min(next_height + chunk - 1, end)
            batch = count_txs_batch(next_height, chunk_end)
            for height in sorted(batch):
                committed_txs += batch[height]
                next_height = height + 1
                if committed_txs >= expected_txs:
                    # stop at the height that actually hit the threshold, not
                    # the (possibly further-extended) outer `end` -
                    # returning `end` here overshoots and lets downstream
                    # block-stats recount blocks past the drain point,
                    # inflating totals past what was sent.
                    return height, committed_txs
                if committed_txs != stall_committed_txs:
                    stall_committed_txs = committed_txs
                    stall_since_height = height
                elif (
                    committed_txs > 0
                    and stall_blocks is not None
                    and height - stall_since_height >= stall_blocks
                ):
                    print(
                        f"commits stalled at {committed_txs}/{expected_txs} for "
                        f"{stall_blocks} blocks (height={height}) - giving up early",
                        file=sys.stderr,
                    )
                    return height, committed_txs
            if not batch:
                # A batch call that reports zero heights (e.g. a partial
                # /blockchain page) leaves next_height unadvanced - sleep so a
                # persistently empty response backs off instead of busy-looping.
                time.sleep(0.2)

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

        if next_height > end:
            current = get_height()
            if current > end:
                end = current
            else:
                time.sleep(0.2)


def wait_for_committed_txs(
    rpc,
    start,
    end,
    expected_txs,
    timeout=LOAD_COMMIT_TIMEOUT,
    initial_committed=0,
    stall_blocks=STALL_BLOCKS,
):
    """Extend the sample until all generated Cosmos txs are committed."""
    return _wait_for_committed(
        lambda: block_height(rpc),
        lambda lo, hi: {h: n for h, (n, _) in blockchain_range(lo, hi, rpc).items()},
        start,
        end,
        expected_txs,
        timeout,
        stall_blocks=stall_blocks,
        initial_committed=initial_committed,
    )


def wait_for_committed_eth_txs(
    json_rpc, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT
):
    """Extend the sample until all generated Ethereum txs are committed."""
    return _wait_for_committed(
        lambda: eth_block_number(json_rpc),
        lambda lo, hi: {
            h: len(block_eth(h, json_rpc)["transactions"]) for h in range(lo, hi + 1)
        },
        start,
        end,
        expected_txs,
        timeout,
        chunk=WAIT_SCAN_CHUNK_ETH,
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

    # Right after a run's load is reported as committed, a few accounts' last
    # txs can still be catching up (async broadcast/retry lag), so an
    # immediate scan can see a stale nonce on a handful of accounts even
    # though the run truly settled. Retry briefly before treating it as a
    # real divergence.
    nonces = None
    for attempt in range(NONCE_SETTLE_RETRIES):
        nonces = set(query_sender_nonces(cfg, physical_start, physical_end).values())
        if len(nonces) == 1:
            break
        if attempt < NONCE_SETTLE_RETRIES - 1:
            time.sleep(NONCE_SETTLE_RETRY_DELAY)
    if len(nonces) != 1:
        values = ", ".join(str(value) for value in sorted(nonces))
        raise ValueError(
            f"benchmark sender accounts have different nonces ({values}); "
            "pass --nonce explicitly or use a fresh account range"
        )
    return nonces.pop()


def query_sender_nonces(cfg, physical_start, physical_end):
    """Return {account_index: on-chain nonce} for every physical sender in
    [physical_start, physical_end].

    One HTTP round trip per account, so a large range is fanned out across a
    thread pool rather than queried serially - at 20k+ accounts, a tunneled
    RPC's per-request latency alone would otherwise stretch this into tens of
    minutes.
    """
    w3 = web3.Web3(web3.HTTPProvider(cfg.primary.json_rpc))
    indices = range(physical_start, physical_end + 1)
    with ThreadPoolExecutor(max_workers=NONCE_QUERY_WORKERS) as pool:
        nonces = pool.map(
            lambda i: w3.eth.get_transaction_count(gen_account(cfg.global_seq, i).address),
            indices,
        )
        return dict(zip(indices, nonces))


def _send_and_report_failures(
    txs, rpcs, logical_num_accounts=None, send_workers=1, **send_kwargs
):
    """Round-robin send `txs`, warning (not raising) on any that never reached
    the mempool: the sender already retried them, and the caller still needs
    to wait out whatever did land rather than abort on a partial send.

    `logical_num_accounts` is the raw account count from `gen()`'s layout,
    used only to split `txs` across `send_workers` processes when >1 - it is
    independent of `send_kwargs["num_accounts"]`, which carries the separate
    sender-affinity/ordering signal `send_round_robin` needs (None under
    unique-per-tx).
    """
    if send_workers > 1:
        affinity_num_accounts = send_kwargs.pop("num_accounts", None)
        failed = send_multiprocess(
            txs,
            rpcs,
            logical_num_accounts,
            num_workers=send_workers,
            nonce_ordered=affinity_num_accounts is not None,
            **send_kwargs,
        )
    else:
        failed = asyncio.run(send_round_robin(txs, rpcs, **send_kwargs))
    if failed:
        print(
            f"warning: {failed}/{len(txs)} txs never reached the mempool "
            "(send retries exhausted)",
            file=sys.stderr,
        )
    return failed


def _reconcile_nonce_gaps(cfg, txs, start, end, num_accounts, base_nonce):
    """Heal per-account nonce gaps left by fully-async sending under `reuse`.

    Async sends race CometBFT's nonce-admission check, so a handful of
    accounts can end up with an unconfirmed tail even though nothing failed
    at send time. Query each account's on-chain nonce and resend, in order,
    every account behind `base_nonce + cfg.num_txs`.

    Only applies when `cfg.sender_strategy == "reuse"`, `cfg.mode ==
    "cosmos"`, and `cfg.batch_size == 1` - the exact conditions under which a
    send can race another send from the same account. Returns `(still_missing,
    healed)`.
    """
    if not (
        cfg.sender_strategy == "reuse" and cfg.mode == "cosmos" and cfg.batch_size == 1
    ):
        return 0, 0

    nonces = query_sender_nonces(cfg, start, end)
    # An account already behind base_nonce (e.g. a straggler left over from a
    # prior run) has confirmed none of *this* run's txs - clamp to 0 rather
    # than letting a negative offset wrap the flat txs[] index below.
    missing = {
        i - start: max(0, nonces[i] - base_nonce)
        for i in range(start, end + 1)
        if nonces[i] - base_nonce < cfg.num_txs
    }
    if not missing:
        return 0, 0

    total_gap = sum(cfg.num_txs - confirmed for confirmed in missing.values())
    print(
        f"reconciling {total_gap} gapped nonces across {len(missing)} accounts...",
        file=sys.stderr,
    )
    rpcs = cfg.rpc_candidates
    still_missing = asyncio.run(
        resend_missing_nonces(
            txs,
            lambda account_index: rpcs[account_index % len(rpcs)],
            num_accounts,
            cfg.num_txs,
            missing,
            mode=cfg.mode,
            conn_per_host=cfg.send_conn_per_host,
            n_hosts=len(rpcs),
        )
    )
    if still_missing:
        print(
            f"warning: {still_missing} txs still missing a nonce after reconciliation",
            file=sys.stderr,
        )
    return still_missing, total_gap - still_missing


def _run_warmup(cfg, start, nonce, num_accounts):
    """Send `cfg.warmup_txs` throwaway txs per account and wait for them to
    commit, so the measured run isn't paying for cold mempool/connection-pool
    state. No-op when warmup_txs is 0. Returns the nonce the real load
    generation should resume from.

    Skipped under unique-per-tx: genesis only funds num_accounts * num_txs
    physical senders, and the main load already signs every one of them at
    every offset - there is no disjoint sub-range left for warm-up to use
    without colliding with a sender the main load expects to still be at
    nonce 0.
    """
    if not getattr(cfg, "warmup_txs", 0):
        return nonce
    if cfg.sender_strategy == "unique-per-tx":
        print(
            f"skipping warm-up ({cfg.warmup_txs} tx/account configured): "
            "not supported under unique-per-tx",
            file=sys.stderr,
        )
        return nonce

    print(f"warming up with {cfg.warmup_txs} tx/account...", file=sys.stderr)
    txs = gen_from_config(cfg, num_accounts, cfg.warmup_txs, start, nonce)
    if cfg.mode == "eth":
        send_rpcs = cfg.json_rpc_candidates
        poll_rpcs = cfg.primary.json_rpc_candidates
        get_height = eth_block_number
        wait_for_commits = wait_for_committed_eth_txs
    else:
        send_rpcs = cfg.rpc_candidates
        poll_rpcs = cfg.primary.rpc_candidates
        get_height = block_height
        wait_for_commits = wait_for_committed_txs

    load_start = get_height(poll_rpcs)
    failed = _send_and_report_failures(
        txs,
        send_rpcs,
        batch_size=cfg.send_batch_size,
        batch_interval=cfg.send_interval,
        mode=cfg.mode,
        num_accounts=sender_affinity_accounts(cfg.sender_strategy, num_accounts),
        conn_per_host=cfg.send_conn_per_host,
    )
    load_end = get_height(poll_rpcs)
    wait_for_commits(
        poll_rpcs,
        load_start,
        load_end,
        len(txs) - failed,
        timeout=cfg.commit_timeout,
    )
    print("warm-up committed", file=sys.stderr)
    return nonce + cfg.warmup_txs


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

    if nonce is None:
        nonce = current_sender_nonce(cfg, start, end)
        print(f"using current sender nonce {nonce}", file=sys.stderr)

    # Warm-up must run before the cache-hit check below, even when a payload is
    # cached: the cache was signed assuming warm-up already advanced the nonce,
    # but each run starts from a fresh chain (nonce back at 0), so skipping
    # warm-up on a cache hit would compare against a stale, pre-warm-up nonce.
    nonce = _run_warmup(cfg, start, nonce, num_accounts)

    if cached_payload is not None:
        # The cache only stays valid if the chain's actual current nonce still
        # matches the nonce it was signed against - a torn-down-and-reinitialized
        # chain (fresh nonce 0) replaying a cache signed at a later nonce fails
        # every tx's CheckTx instead of raising here with a clear cause.
        cached_nonce = cached_payload.get("nonce")
        if cached_nonce != nonce:
            raise ValueError(
                f"--txs-cache {txs_cache} was signed against nonce {cached_nonce}, "
                f"but the chain's senders are currently at nonce {nonce}; "
                "remove the stale cache file or point --txs-cache elsewhere"
            )
        txs = cached_payload["txs"]
        print(f"loaded {len(txs)} cached {cfg.mode} txs from {txs_cache}", file=sys.stderr)
    else:
        print("generating txs...", file=sys.stderr)
        txs = gen_from_config(cfg, num_accounts, cfg.num_txs, start, nonce)
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
                ujson.dumps(
                    {"num_accounts": num_accounts, "num_txs": cfg.num_txs, "nonce": nonce, "txs": txs}
                )
            )
            tmp_path.replace(txs_cache_path)
            print(f"wrote tx cache to {txs_cache}", file=sys.stderr)

    stats_buffer = io.StringIO() if capture_stats else None
    stats_out = Tee(sys.stdout, stats_buffer) if capture_stats else sys.stdout

    if cfg.mode == "eth":
        load_start = eth_block_number(cfg.primary.json_rpc_candidates)
        print("sending txs...", file=sys.stderr)
        failed = _send_and_report_failures(
            txs,
            cfg.json_rpc_candidates,
            batch_size=cfg.send_batch_size,
            batch_interval=cfg.send_interval,
            mode=cfg.mode,
            num_accounts=sender_affinity_accounts(cfg.sender_strategy, num_accounts),
            probe_batches=probe_batches,
            conn_per_host=cfg.send_conn_per_host,
            logical_num_accounts=num_accounts,
            send_workers=cfg.send_workers,
        )
        load_end = eth_block_number(cfg.primary.json_rpc_candidates)
        load_end, committed_txs = wait_for_committed_eth_txs(
            cfg.primary.json_rpc_candidates,
            load_start,
            load_end,
            len(txs) - failed,
            timeout=cfg.commit_timeout,
        )
        summary = dump_eth_block_stats(
            stats_out,
            json_rpc=cfg.primary.json_rpc_candidates,
            start=load_start,
            end=load_end,
        )
        print(f"committed_eth_txs {committed_txs}/{len(txs)}")
    else:
        is_app_mempool = (getattr(cfg.primary, "node_config", None) or {}).get("mempool.type") == "app"
        mempool_monitor = MempoolMonitor(
            cfg.primary.rpc_candidates,
            json_rpc=cfg.primary.json_rpc_candidates[0] if is_app_mempool else None,
        )
        stm_monitor = BlockSTMMonitor(cfg.primary.rpc_candidates, cfg.telemetry)
        prom_baseline_text = _fetch_prometheus(cfg.telemetry)
        consensus_baseline = scrape_consensus_raw(prom_baseline_text)
        consensus_health_baseline = scrape_consensus_health_raw(prom_baseline_text)
        mempool_health_baseline = scrape_mempool_health_raw(prom_baseline_text)
        sdk_metrics_baseline = scrape_sdk_tx_metrics(
            fetch_sdk_prometheus_text(cfg.sdk_metrics)
        )
        cronos_mempool_baseline = scrape_cronos_mempool_raw(
            fetch_sdk_prometheus_text(cfg.sdk_metrics)
        )
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

        load_start = block_height(cfg.primary.rpc_candidates)
        mempool_monitor.start()
        stm_monitor.start()
        committed_txs = 0
        failed = 0
        try:
            print("sending txs...", file=sys.stderr)
            failed = _send_and_report_failures(
                txs,
                cfg.rpc_candidates,
                batch_size=cfg.send_batch_size,
                batch_interval=cfg.send_interval,
                num_accounts=sender_affinity_accounts(cfg.sender_strategy, num_accounts),
                probe_batches=probe_batches,
                conn_per_host=cfg.send_conn_per_host,
                logical_num_accounts=num_accounts,
                send_workers=cfg.send_workers,
            )
            load_end = block_height(cfg.primary.rpc_candidates)
            load_end, committed_txs = wait_for_committed_txs(
                cfg.primary.rpc_candidates,
                load_start,
                load_end,
                len(txs) - failed,
                timeout=cfg.commit_timeout,
            )
            # Query nonces only after the chain has stopped making progress on
            # its own (threshold hit or stalled) - checking right after send
            # would see nothing committed yet and resend every account.
            still_missing, healed = _reconcile_nonce_gaps(
                cfg, txs, start, end, num_accounts, nonce
            )
            failed += still_missing
            if healed:
                # Rescan from the original load_start rather than resuming from
                # "now" - most of the backlog the first wait gave up on keeps
                # committing in bursts (not steadily), often more than
                # STALL_BLOCKS apart, while reconciliation's resends are still
                # in flight over the network. Resuming from "now" skips exactly
                # that window, silently losing whatever committed during it.
                # Disable the stall giveup too, for the same reason - a real
                # commit burst can trail a long quiet gap; only the timeout
                # should end this wait.
                load_end = block_height(cfg.primary.rpc_candidates)
                load_end, committed_txs = wait_for_committed_txs(
                    cfg.primary.rpc_candidates,
                    load_start,
                    load_end,
                    len(txs) - failed,
                    timeout=cfg.commit_timeout,
                    stall_blocks=None,
                )
        finally:
            mempool_monitor.stop()
            stm_monitor.stop()

        summary = dump_block_stats(
            stats_out,
            rpc=cfg.primary.rpc_candidates,
            json_rpc=cfg.primary.json_rpc_candidates,
            telemetry=cfg.telemetry,
            start=load_start,
            end=load_end,
            mempool_data=mempool_monitor.data,
            stm_data=stm_monitor.data,
            consensus_baseline=consensus_baseline,
            consensus_health_baseline=consensus_health_baseline,
            mempool_health_baseline=mempool_health_baseline,
            sdk_metrics=cfg.sdk_metrics,
            sdk_metrics_baseline=sdk_metrics_baseline,
            cronos_mempool_baseline=cronos_mempool_baseline,
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
