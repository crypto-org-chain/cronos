from types import SimpleNamespace

from remote_benchmark import ramp as ramp_module
from remote_benchmark.ramp import ramp_test, stage_verdict


def _checkpoints(tps_values):
    return [{"tps": v} for v in tps_values]


def test_stage_verdict_ok_when_achieved_meets_target_and_no_trend_collapse(monkeypatch):
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {})
    monkeypatch.setattr(
        ramp_module, "soak_verdict", lambda trends, checkpoints, telemetry: {"ok": True, "reasons": []}
    )

    result = stage_verdict(1000, _checkpoints([950, 980, 990]), telemetry=None, accept_frac=0.85)

    assert result["ok"] is True
    assert result["achieved_tps"] == 980


def test_stage_verdict_fails_when_achieved_tps_is_well_below_target(monkeypatch):
    # soak_verdict passes (its own trend is flat) but the stage never got
    # anywhere near the rate we asked for - the achieved-vs-target floor is
    # the only thing that can catch this.
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {})
    monkeypatch.setattr(
        ramp_module, "soak_verdict", lambda trends, checkpoints, telemetry: {"ok": True, "reasons": []}
    )

    result = stage_verdict(1000, _checkpoints([300, 320, 310]), telemetry=None, accept_frac=0.85)

    assert result["ok"] is False
    assert result["achieved_tps"] == 310


def test_stage_verdict_fails_when_soak_verdict_flags_within_stage_collapse(monkeypatch):
    # Achieved tps at the tail looks fine, but soak_verdict's own trend gate
    # caught a mid-stage collapse - that must still fail the stage.
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {"tps": -50.0})
    monkeypatch.setattr(
        ramp_module,
        "soak_verdict",
        lambda trends, checkpoints, telemetry: {"ok": False, "reasons": ["throughput fell"]},
    )

    result = stage_verdict(1000, _checkpoints([990, 950, 900]), telemetry=None, accept_frac=0.85)

    assert result["ok"] is False
    assert result["achieved_tps"] == 950


def _fake_cfg():
    return SimpleNamespace(telemetry=None)


def test_ramp_test_stops_at_first_failing_stage_and_reports_prior_rate(monkeypatch):
    calls = []

    def fake_run_stage(cfg, start, end, num_accounts, rate, duration, checkpoint_interval, nonce, send_workers=1):
        calls.append(rate)
        tps = rate if rate < 3000 else rate * 0.5  # stage at 3000 fails to keep up
        return _checkpoints([tps, tps]), 42, nonce + 10, 0

    monkeypatch.setattr(ramp_module, "run_ramp_stage", fake_run_stage)
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {})
    monkeypatch.setattr(
        ramp_module, "soak_verdict", lambda trends, checkpoints, telemetry: {"ok": True, "reasons": []}
    )

    result = ramp_test(
        _fake_cfg(),
        start=0,
        end=99,
        start_rate=1000,
        rate_step=1000,
        stage_duration=60,
        checkpoint_interval=15,
        nonce=0,
        accept_frac=0.85,
    )

    assert calls == [1000, 2000, 3000]
    assert result["sustained_rate"] == 2000
    assert [s["rate"] for s in result["stages"]] == [1000, 2000, 3000]
    assert [s["ok"] for s in result["stages"]] == [True, True, False]


def test_ramp_test_reports_max_rate_when_every_stage_holds(monkeypatch):
    def fake_run_stage(cfg, start, end, num_accounts, rate, duration, checkpoint_interval, nonce, send_workers=1):
        return _checkpoints([rate, rate]), 0, nonce + 10, 0

    monkeypatch.setattr(ramp_module, "run_ramp_stage", fake_run_stage)
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {})
    monkeypatch.setattr(
        ramp_module, "soak_verdict", lambda trends, checkpoints, telemetry: {"ok": True, "reasons": []}
    )

    result = ramp_test(
        _fake_cfg(),
        start=0,
        end=99,
        start_rate=1000,
        rate_step=1000,
        stage_duration=60,
        checkpoint_interval=15,
        nonce=0,
        max_rate=2000,
    )

    assert result["sustained_rate"] == 2000
    assert [s["rate"] for s in result["stages"]] == [1000, 2000]


def test_ramp_test_reports_no_sustained_rate_when_first_stage_fails(monkeypatch):
    def fake_run_stage(cfg, start, end, num_accounts, rate, duration, checkpoint_interval, nonce, send_workers=1):
        return _checkpoints([50, 50]), 0, nonce + 10, 0

    monkeypatch.setattr(ramp_module, "run_ramp_stage", fake_run_stage)
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {})
    monkeypatch.setattr(
        ramp_module, "soak_verdict", lambda trends, checkpoints, telemetry: {"ok": True, "reasons": []}
    )

    result = ramp_test(
        _fake_cfg(),
        start=0,
        end=99,
        start_rate=1000,
        rate_step=1000,
        stage_duration=60,
        checkpoint_interval=15,
        nonce=0,
    )

    assert result["sustained_rate"] is None
    assert len(result["stages"]) == 1
    assert result["stages"][0]["ok"] is False


def test_ramp_test_carries_nonce_forward_between_stages(monkeypatch):
    seen_nonces = []

    def fake_run_stage(cfg, start, end, num_accounts, rate, duration, checkpoint_interval, nonce, send_workers=1):
        seen_nonces.append(nonce)
        return _checkpoints([rate, rate]), 0, nonce + 10, 0

    monkeypatch.setattr(ramp_module, "run_ramp_stage", fake_run_stage)
    monkeypatch.setattr(ramp_module, "fit_trends", lambda checkpoints: {})
    monkeypatch.setattr(
        ramp_module, "soak_verdict", lambda trends, checkpoints, telemetry: {"ok": True, "reasons": []}
    )

    ramp_test(
        _fake_cfg(),
        start=0,
        end=99,
        start_rate=1000,
        rate_step=1000,
        stage_duration=60,
        checkpoint_interval=15,
        nonce=5,
        max_rate=3000,
    )

    assert seen_nonces == [5, 15, 25]
