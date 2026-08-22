"""Ramp testing: find the sustained tx/s ceiling.

Runs a series of fixed-rate stages (each a short soak, see `soak.py`),
starting low and stepping the rate up, stopping as soon as a stage fails to
sustain its target rate. The rate of the last stage that held is the
network's discovered ceiling.
"""

import asyncio
import logging
import sys
import time
from statistics import median

from .monitor import MempoolMonitor
from .runner import gen_from_config
from .soak import (
    CheckpointSampler,
    fit_trends,
    soak_tx_supply,
    soak_verdict,
    wait_out_soak_duration,
)
from .transaction import send_multiprocess, send_round_robin, sender_affinity_accounts

log = logging.getLogger(__name__)


def run_ramp_stage(cfg, start, end, num_accounts, rate, duration, checkpoint_interval, nonce, send_workers=1):
    """Send at `rate` tx/s for `duration` seconds, sampling checkpoints and
    mempool pressure throughout. Returns (checkpoints, mempool_peak, next_nonce,
    failed_sends); `next_nonce` is `nonce` advanced past every tx this stage
    generated, for the caller to chain into the next stage.

    `send_workers > 1` sends via `send_multiprocess`: one asyncio event loop
    per OS process is CPU-bound well below the rates this test wants to
    probe, so multiple worker processes are needed to push past that ceiling.
    """
    batch_interval = 1.0
    num_txs, batch_size = soak_tx_supply(rate, duration, num_accounts, batch_interval, cfg.batch_size)
    txs = gen_from_config(cfg, num_accounts, num_txs, start, nonce)
    affinity_num_accounts = sender_affinity_accounts(cfg.sender_strategy, num_accounts)

    is_app_mempool = (getattr(cfg.primary, "node_config", None) or {}).get("mempool.type") == "app"
    mempool_monitor = MempoolMonitor(
        cfg.primary.rpc_candidates,
        json_rpc=cfg.primary.json_rpc_candidates[0] if is_app_mempool else None,
    )
    sampler = CheckpointSampler(cfg.primary.rpc, cfg.telemetry, checkpoint_interval)

    started = time.monotonic()
    sampler.start()
    mempool_monitor.start()
    try:
        send_kwargs = {
            "batch_size": batch_size,
            "batch_interval": batch_interval,
            "deadline_s": duration,
            "conn_per_host": cfg.send_conn_per_host,
        }
        if send_workers > 1:
            failed = send_multiprocess(
                txs,
                cfg.rpc_candidates,
                num_accounts,
                num_workers=send_workers,
                nonce_ordered=affinity_num_accounts is not None,
                **send_kwargs,
            )
        else:
            failed = asyncio.run(
                send_round_robin(txs, cfg.rpc_candidates, num_accounts=affinity_num_accounts, **send_kwargs)
            )
        wait_out_soak_duration(started, duration)
    finally:
        sampler.stop()
        mempool_monitor.stop()

    mempool_peak = max((n_txs for n_txs, _ in mempool_monitor.data.values()), default=0)
    return sampler.checkpoints, mempool_peak, nonce + num_txs, failed


def stage_verdict(target_rate, checkpoints, telemetry, accept_frac=0.85):
    """Whether a stage counts as having sustained `target_rate`.

    `soak_verdict`'s trend gates only catch a stage collapsing relative to its
    *own* opening baseline - a stage that never got off the ground (flat, low
    tps throughout) still passes those. The achieved-vs-target floor below is
    what actually answers "did the network keep up with what we asked for."
    """
    trends = fit_trends(checkpoints)
    verdict = soak_verdict(trends, checkpoints, telemetry)
    tps_values = [c["tps"] for c in checkpoints if c["tps"] is not None]
    achieved_tps = median(tps_values) if tps_values else 0.0
    target_met = achieved_tps >= accept_frac * target_rate
    return {
        "ok": verdict["ok"] and target_met,
        "achieved_tps": achieved_tps,
        "target_rate": target_rate,
        "soak_verdict": verdict,
    }


def ramp_test(
    cfg,
    start,
    end,
    start_rate,
    rate_step,
    stage_duration,
    checkpoint_interval,
    nonce,
    max_rate=None,
    accept_frac=0.85,
    send_workers=1,
):
    """Step the send rate up stage by stage while the network keeps up,
    stopping at the first stage that doesn't. Returns
    {"stages": [...], "sustained_rate": float | None}.
    """
    num_accounts = end - start + 1
    rate = start_rate
    stages = []
    sustained_rate = None

    while max_rate is None or rate <= max_rate:
        print(f"stage: target_rate={rate:.1f} tx/s for {stage_duration:.0f}s...", file=sys.stderr)
        checkpoints, mempool_peak, nonce, failed = run_ramp_stage(
            cfg, start, end, num_accounts, rate, stage_duration, checkpoint_interval, nonce, send_workers=send_workers
        )
        verdict = stage_verdict(rate, checkpoints, cfg.telemetry, accept_frac)
        stage = {
            "rate": rate,
            "achieved_tps": verdict["achieved_tps"],
            "mempool_peak": mempool_peak,
            "failed_sends": failed,
            "ok": verdict["ok"],
            "soak_verdict": verdict["soak_verdict"],
        }
        stages.append(stage)
        print(
            f"stage result: target={rate:.1f} achieved={verdict['achieved_tps']:.1f} tx/s "
            f"mempool_peak={mempool_peak} ok={verdict['ok']}",
            file=sys.stderr,
        )
        if not verdict["ok"]:
            break
        sustained_rate = rate
        rate += rate_step

    return {"stages": stages, "sustained_rate": sustained_rate}
