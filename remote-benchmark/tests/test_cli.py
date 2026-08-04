import json
from types import SimpleNamespace

from click import ClickException
from click.testing import CliRunner

from remote_benchmark import cli as cli_module


def _stub_soak_wait(monkeypatch):
    """`soak` holds until --duration elapses so the sampler's last checkpoint
    lands. These tests drive a fake sampler, so the wait is pure delay; the
    returned list records the durations it was asked to wait out."""
    waits = []
    monkeypatch.setattr(
        cli_module,
        "_wait_out_soak_duration",
        lambda _started, duration: waits.append(duration),
    )
    return waits


# One endpoint: check_divergence has nothing to compare and returns None without
# touching the network, so the commands' divergence gate stays out of the way of
# these tests.
_ONE_ENDPOINT = [
    SimpleNamespace(name="node0", rpc="http://node0", json_rpc="http://node0-evm")
]


class FakeMonitor:
    def __init__(self, *_args):
        self.data = {}

    def start(self):
        pass

    def stop(self):
        pass


def test_wait_for_committed_returns_the_height_that_actually_hit_the_threshold():
    # The chain runs ahead to height 205 (get_height) before the block that
    # actually hits expected_txs (202) gets counted. Returning that stale,
    # further-extended `end` instead of the height just counted would make
    # callers re-scan blocks 203-205 for stats, over-counting txs that were
    # never part of this load.
    per_block = {201: 2, 202: 3, 203: 1, 204: 1, 205: 1}
    end, committed = cli_module._wait_for_committed(
        get_height=lambda: 205,
        count_txs=lambda height: per_block.get(height, 0),
        start=200,
        end=201,
        expected_txs=5,
    )

    assert end == 202
    assert committed == 5


def test_current_sender_nonce_rejects_mixed_physical_sender_nonces(monkeypatch):
    requested_addresses = []

    class FakeEth:
        def get_transaction_count(self, address):
            requested_addresses.append(address)
            return {"account-3": 2, "account-4": 3}[address]

    cfg = SimpleNamespace(
        primary=SimpleNamespace(json_rpc="http://node0-evm"),
        global_seq=0,
        num_txs=2,
        sender_strategy="unique-per-tx",
    )
    monkeypatch.setattr(
        cli_module.web3,
        "Web3",
        lambda _provider: SimpleNamespace(eth=FakeEth()),
    )
    monkeypatch.setattr(
        cli_module,
        "gen_account",
        lambda _seq, index: SimpleNamespace(address=f"account-{index}"),
    )

    try:
        cli_module.current_sender_nonce(cfg, 3, 3)
    except ClickException as exc:
        assert "different nonces (2, 3)" in str(exc)
    else:
        raise AssertionError("mixed sender nonces were accepted")

    assert requested_addresses == ["account-3", "account-4"]


def test_current_sender_nonce_num_txs_override_widens_the_sender_range(monkeypatch):
    requested_addresses = []

    cfg = SimpleNamespace(
        primary=SimpleNamespace(json_rpc="http://node0-evm"),
        global_seq=0,
        num_txs=1,
        sender_strategy="unique-per-tx",
    )
    monkeypatch.setattr(
        cli_module.web3,
        "Web3",
        lambda _provider: SimpleNamespace(
            eth=SimpleNamespace(
                get_transaction_count=lambda address: requested_addresses.append(address) or 0
            )
        ),
    )
    monkeypatch.setattr(
        cli_module,
        "gen_account",
        lambda _seq, index: SimpleNamespace(address=f"account-{index}"),
    )

    # cfg.num_txs=1 would only check account-0; the soak's own num_txs=4 is what
    # gen() actually signs from.
    assert cli_module.current_sender_nonce(cfg, 0, 0, num_txs=4) == 0
    assert requested_addresses == ["account-0", "account-1", "account-2", "account-3"]


def test_bench_waits_for_all_generated_txs_to_commit(monkeypatch):
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm", node_exporter=None),
        endpoints=_ONE_ENDPOINT,
        rpcs=["http://node0", "http://node1", "http://node2"],
        global_seq=0,
        num_txs=1,
        sender_strategy="reuse",
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=100,
        msg_version="1.4",
        gas_price=1,
        chain_id=777,
        evm_denom="basetcro",
        mode="cosmos",
        send_batch_size=3,
        send_interval=0,
        telemetry=None,
    )
    heights = iter([171, 173, 174, 175])
    loaded_txs = {174: ["tx-1"], 175: ["tx-2", "tx-3"]}

    async def fake_send(*_args, **_kwargs):
        return 0

    def fake_dump(fp, *, start, end, **_kwargs):
        for height in range(start, end + 1):
            print(f"block {height} txs={len(loaded_txs.get(height, []))}", file=fp)
        if not set(loaded_txs).intersection(range(start, end + 1)):
            print("no_load_period", file=fp)

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module, "gen", lambda *_args, **_kwargs: ["tx-1", "tx-2", "tx-3"]
    )
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "block_height", lambda _rpc: next(heights))
    monkeypatch.setattr(
        cli_module,
        "block_txs",
        lambda height, _rpc: loaded_txs.get(height, []),
        raising=False,
    )
    monkeypatch.setattr(cli_module, "MempoolMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "BlockSTMMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "_fetch_prometheus", lambda _url: "")
    monkeypatch.setattr(cli_module, "scrape_consensus_raw", lambda _text: {})
    monkeypatch.setattr(cli_module, "dump_block_stats", fake_dump)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"],
    )

    assert result.exit_code == 0, result.exception
    assert "block 175 txs=2" in result.output
    assert "committed_cosmos_txs 3/3" in result.output
    assert "no_load_period" not in result.output


def test_bench_does_not_wait_for_txs_that_send_gave_up_on(monkeypatch):
    """A tx whose async_sendtx retries exhaust never reaches the mempool, so
    waiting for it to commit would time out even though nothing else is
    wrong - expected_txs must shrink by the reported failure count instead."""
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm", node_exporter=None),
        endpoints=_ONE_ENDPOINT,
        rpcs=["http://node0", "http://node1", "http://node2"],
        global_seq=0,
        num_txs=1,
        sender_strategy="reuse",
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=100,
        msg_version="1.4",
        gas_price=1,
        chain_id=777,
        evm_denom="basetcro",
        mode="cosmos",
        send_batch_size=3,
        send_interval=0,
        telemetry=None,
    )
    heights = iter([171, 173, 174])
    loaded_txs = {174: ["tx-1", "tx-2"]}

    async def fake_send(*_args, **_kwargs):
        return 1

    def fake_dump(fp, *, start, end, **_kwargs):
        for height in range(start, end + 1):
            print(f"block {height} txs={len(loaded_txs.get(height, []))}", file=fp)

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module, "gen", lambda *_args, **_kwargs: ["tx-1", "tx-2", "tx-3"]
    )
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "block_height", lambda _rpc: next(heights))
    monkeypatch.setattr(
        cli_module,
        "block_txs",
        lambda height, _rpc: loaded_txs.get(height, []),
        raising=False,
    )
    monkeypatch.setattr(cli_module, "MempoolMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "BlockSTMMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "_fetch_prometheus", lambda _url: "")
    monkeypatch.setattr(cli_module, "scrape_consensus_raw", lambda _text: {})
    monkeypatch.setattr(cli_module, "dump_block_stats", fake_dump)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"],
    )

    assert result.exit_code == 0, result.exception
    assert "1/3 txs never reached the mempool" in result.output
    assert "committed_cosmos_txs 2/3" in result.output


def test_eth_bench_waits_for_generated_txs_to_commit(monkeypatch):
    cfg = SimpleNamespace(
        primary=SimpleNamespace(json_rpc="http://anvil"),
        endpoints=_ONE_ENDPOINT,
        json_rpcs=["http://anvil"],
        global_seq=0,
        num_txs=5,
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=1,
        msg_version="1.4",
        gas_price=1,
        chain_id=31337,
        evm_denom="basetcro",
        mode="eth",
        sender_strategy="reuse",
        send_batch_size=50,
        send_interval=0,
    )
    heights = iter([1037, 1037, 1038])
    loaded_txs = {1038: [f"tx-{i}" for i in range(50)]}

    async def fake_send(*_args, **_kwargs):
        return 0

    def fake_dump(fp, *, start, end, **_kwargs):
        for height in range(start, end + 1):
            print(f"block {height} txs={len(loaded_txs.get(height, []))}", file=fp)
        if not set(loaded_txs).intersection(range(start, end + 1)):
            print("no_load_period", file=fp)

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "current_sender_nonce", lambda *_args: 10)
    monkeypatch.setattr(
        cli_module, "gen", lambda *_args, **_kwargs: list(loaded_txs[1038])
    )
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "eth_block_number", lambda _rpc: next(heights))
    monkeypatch.setattr(
        cli_module,
        "block_eth",
        lambda height, _rpc: {"transactions": loaded_txs.get(height, [])},
        raising=False,
    )
    monkeypatch.setattr(cli_module, "dump_eth_block_stats", fake_dump)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "1", "10"],
    )

    assert result.exit_code == 0, result.exception
    assert "using current sender nonce 10" in result.output
    assert "block 1038 txs=50" in result.output
    assert "committed_eth_txs 50/50" in result.output
    assert "no_load_period" not in result.output


def test_bench_fails_when_not_all_generated_txs_commit(monkeypatch):
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm", node_exporter=None),
        endpoints=_ONE_ENDPOINT,
        rpcs=["http://node0"],
        global_seq=0,
        num_txs=1,
        sender_strategy="reuse",
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=1,
        msg_version="1.4",
        gas_price=1,
        chain_id=777,
        evm_denom="basetcro",
        mode="cosmos",
        send_batch_size=2,
        send_interval=0,
        telemetry=None,
    )

    async def fake_send(*_args, **_kwargs):
        return 0

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "gen", lambda *_args, **_kwargs: ["tx-1", "tx-2"])
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "block_height", lambda _rpc: 10)
    monkeypatch.setattr(
        cli_module,
        "wait_for_committed_txs",
        lambda *_args, **_kwargs: (11, 1),
    )
    monkeypatch.setattr(cli_module, "MempoolMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "BlockSTMMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "_fetch_prometheus", lambda _url: "")
    monkeypatch.setattr(cli_module, "scrape_consensus_raw", lambda _text: {})
    monkeypatch.setattr(cli_module, "dump_block_stats", lambda *_args, **_kwargs: None)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "2"],
    )

    assert result.exit_code != 0
    assert "committed_cosmos_txs 1/2" in result.output
    assert "1/2 Cosmos transactions committed" in result.output


def test_fund_mode_override_uses_cosmos_for_eth_config(monkeypatch):
    class FakeAccount:
        address = "0x0000000000000000000000000000000000000001"

        def sign_transaction(self, tx):
            return SimpleNamespace(rawTransaction=tx["nonce"])

    class FakeEth:
        def __init__(self):
            self.committed_nonce = 0

        def get_transaction_count(self, _address):
            return self.committed_nonce

        def send_raw_transaction(self, _raw):
            raise AssertionError("raw Ethereum funding transport was used")

    fake_eth = FakeEth()
    cfg = SimpleNamespace(
        primary=SimpleNamespace(json_rpc="http://unused", rpc="http://cosmos-rpc"),
        global_seq=0,
        gas_price=1,
        chain_id=777,
        mode="eth",
        num_txs=1,
        sender_strategy="reuse",
        msg_version="1.4",
        evm_denom="basetcro",
    )
    posts = []

    def fake_post(url, json):
        posts.append((url, json))
        fake_eth.committed_nonce = 2
        return SimpleNamespace(json=lambda: {"result": {"code": 0}})

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module.web3,
        "Web3",
        lambda _provider: SimpleNamespace(eth=fake_eth),
    )
    monkeypatch.setattr(cli_module, "gen_account", lambda _seq, _index: FakeAccount())
    monkeypatch.setattr(cli_module, "build_cosmos_tx", lambda *args, **kwargs: "raw")
    monkeypatch.setattr(cli_module.requests, "post", fake_post)

    result = CliRunner().invoke(
        cli_module.cli,
        [
            "fund",
            "--config",
            "unused.yaml",
            "--mode",
            "cosmos",
            "--batch-size",
            "2",
            "1",
            "2",
        ],
    )

    assert result.exit_code == 0, result.exception
    assert len(posts) == 1


def test_sweep_passes_explicit_nonce_only_to_the_first_cell_that_runs(monkeypatch, tmp_path):
    cfg = SimpleNamespace(mode="cosmos", endpoints=_ONE_ENDPOINT)
    matrix_path = tmp_path / "matrix.json"
    matrix_path.write_text('{"axes": {"workers": [8, 16]}}')

    run_nonces = []

    def fake_run_bench_once(_cfg, nonce, _probe_batches, _start, _end, capture_stats):
        run_nonces.append(nonce)
        return {
            "mode": "cosmos", "load_start": 1, "load_end": 2, "stats_text": "",
            "summary": {"total_counted_txs": 1, "total_failed_txs": 0, "gas_utilizations": [0.9]},
            "committed_txs": 1, "expected_txs": 1,
        }

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "_run_bench_once", fake_run_bench_once)
    monkeypatch.setattr(cli_module, "build_run_record", lambda **_kwargs: {})
    monkeypatch.setattr(cli_module, "write_run_record", lambda *_args: None)

    result = CliRunner().invoke(
        cli_module.cli,
        ["sweep", "--config", "unused.yaml", "--nonce", "5", "--results-dir", str(tmp_path / "out"),
         str(matrix_path), "1", "3"],
    )

    assert result.exit_code == 0, result.exception
    # Only the first cell that actually runs gets the explicit nonce; later
    # cells pass None so _run_bench_once re-queries the live chain nonce,
    # since earlier cells already consumed nonces by sending transactions.
    assert run_nonces == [5, None]


def test_soak_batch_size_for_a_reachable_rate():
    # 500 EVM tx/s at 200 EVM txs per wire tx = 2.5 -> 2 wire txs per second
    assert cli_module._soak_batch_size(500, 1.0, 200) == 2
    # unbatched (eth) mode paces one wire tx per EVM tx
    assert cli_module._soak_batch_size(50, 1.0, 1) == 50


def test_soak_batch_size_rejects_rate_below_the_batch_size_floor():
    # rounding 50/200 down to 0 wire txs used to be floored at 1, silently
    # sending 200 EVM tx/s for a 50 tx/s target.
    try:
        cli_module._soak_batch_size(50, 1.0, 200)
    except ClickException as exc:
        assert "below the 200 tx/s floor" in str(exc)
    else:
        raise AssertionError("a rate below the batch_size floor was accepted")


def test_soak_batch_size_warns_when_the_rate_is_only_reachable_by_rounding(capsys):
    # 1.4 wire txs/s isn't sendable; the effective rate is 1 tx/s, not 1.4.
    assert cli_module._soak_batch_size(1.4, 1.0, 1) == 1

    assert "using 1 tx/s" in capsys.readouterr().err


def test_soak_batch_size_does_not_warn_for_an_exact_rate(capsys):
    assert cli_module._soak_batch_size(50, 1.0, 1) == 50

    assert capsys.readouterr().err == ""


def test_check_soak_duration_rejects_a_run_too_short_for_two_checkpoints():
    # 45s at the 30s default interval yields one checkpoint, so no trend can be
    # fitted — worth catching before the soak burns its whole duration.
    try:
        cli_module._check_soak_duration(45.0, 30.0)
    except ClickException as exc:
        assert "at least 60s" in str(exc)
    else:
        raise AssertionError("a duration too short for two checkpoints was accepted")


def test_check_soak_duration_accepts_exactly_two_checkpoints():
    cli_module._check_soak_duration(60.0, 30.0)


def test_soak_paces_on_the_effective_batch_size_not_the_configured_one(monkeypatch):
    # gen batches only within one account, so with 12 txs/account a wire tx
    # carries 12 EVM txs, not the configured batch_size=100. Pacing on 100 would
    # send 1 wire tx/s (12 tx/s) for a 100 tx/s target.
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm", node_exporter=None),
        endpoints=_ONE_ENDPOINT,
        rpcs=["http://node0"],
        global_seq=0,
        num_txs=1,
        sender_strategy="reuse",
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=100,
        msg_version="1.4",
        gas_price=1,
        chain_id=777,
        evm_denom="basetcro",
        mode="cosmos",
        telemetry=None,
    )
    captured = {}

    class FakeSampler:
        def __init__(self, *_args):
            self.checkpoints = [
                {
                    "elapsed_s": t,
                    "height": 100 + t,
                    "tps": 100.0,
                    "avg_block_time_ms": 500.0,
                    "rss_bytes": 1_000,
                }
                for t in (0, 30, 60)
            ]

        def start(self):
            pass

        def stop(self):
            pass

    async def fake_send(_txs, _rpcs, **kwargs):
        captured.update(kwargs)
        return 0

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "gen", lambda *_args, **_kwargs: ["tx"] * 200)
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "CheckpointSampler", FakeSampler)
    waits = _stub_soak_wait(monkeypatch)

    result = CliRunner().invoke(
        cli_module.cli,
        ["soak", "--config", "unused.yaml", "--nonce", "0", "--rate", "100",
         "--duration", "60", "1", "1000"],
    )

    assert result.exit_code == 0, result.exception
    # 1000 accounts x 12 txs each packs 12 EVM txs per wire tx, so 8 wire txs/s
    # is the closest whole-wire-tx pacing to the 100 tx/s target.
    assert captured["batch_size"] == 8
    # --duration is a wall-clock cap on the send loop, not just a sizing input.
    assert captured["deadline_s"] == 60.0
    # and the sampler is not stopped until that wall clock is up, so the final
    # checkpoint is recorded even when the paced sender drains early.
    assert waits == [60.0]


def test_soak_tx_supply_covers_the_full_duration():
    # 100 tx/s over 60s across 100 accounts sizes to 60 txs/account = 100 wire
    # txs, but pacing rounds up to 2 wire txs/s = 120 needed: the sender used to
    # run dry around t=50s, the sampler was stopped before the final checkpoint
    # interval closed, and the soak failed for having only one checkpoint.
    num_txs, batch_size = cli_module._soak_tx_supply(100, 60, 100, 1.0, 100)

    # Sizing settles at 120 txs/account, which packs 100 per wire tx into 200
    # wire txs — more than the 1 wire tx/s x 60s the pacing then consumes.
    assert (num_txs, batch_size) == (120, 1)


def test_wait_out_soak_duration_waits_only_for_what_is_left(monkeypatch):
    slept = []
    monkeypatch.setattr(
        cli_module, "time", SimpleNamespace(monotonic=lambda: 100.0, sleep=slept.append)
    )

    cli_module._wait_out_soak_duration(80.0, 60.0)  # 20s of 60s elapsed
    assert slept == [40.0]

    slept.clear()
    cli_module._wait_out_soak_duration(30.0, 60.0)  # already past duration
    assert slept == []


def test_soak_checks_nonces_for_the_soak_computed_tx_count(monkeypatch):
    # Under unique-per-tx the per-account tx count sets the physical sender
    # range, and the soak derives its own from rate x duration: cfg.num_txs=1
    # would validate one account while gen signs from 6000.
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm", node_exporter=None),
        endpoints=_ONE_ENDPOINT,
        rpcs=["http://node0"],
        global_seq=0,
        num_txs=1,
        sender_strategy="unique-per-tx",
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=100,
        msg_version="1.4",
        gas_price=1,
        chain_id=777,
        evm_denom="basetcro",
        mode="cosmos",
        telemetry=None,
    )
    nonce_calls = []
    gen_calls = []

    class FakeSampler:
        def __init__(self, *_args):
            self.checkpoints = [
                {
                    "elapsed_s": t,
                    "height": 100 + t,
                    "tps": 100.0,
                    "avg_block_time_ms": 500.0,
                    "rss_bytes": 1_000,
                }
                for t in (0, 30, 60)
            ]

        def start(self):
            pass

        def stop(self):
            pass

    async def fake_send(*_args, **_kwargs):
        return 0

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module,
        "current_sender_nonce",
        lambda _cfg, start, end, num_txs=None: nonce_calls.append(num_txs) or 0,
    )
    monkeypatch.setattr(
        cli_module,
        "gen",
        lambda _seq, _accounts, num_txs, *_args, **_kwargs: gen_calls.append(num_txs) or ["tx"],
    )
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "CheckpointSampler", FakeSampler)
    _stub_soak_wait(monkeypatch)

    result = CliRunner().invoke(
        cli_module.cli,
        ["soak", "--config", "unused.yaml", "--rate", "100", "--duration", "60", "1", "10"],
    )

    assert result.exit_code == 0, result.exception
    assert nonce_calls == gen_calls == [600]


def test_fund_exits_non_zero_when_the_broadcast_is_rejected(monkeypatch):
    class FakeAccount:
        address = "0x0000000000000000000000000000000000000001"

        def sign_transaction(self, tx):
            return SimpleNamespace(rawTransaction=tx["nonce"])

    class FakeEth:
        def get_transaction_count(self, _address):
            return 0

    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm"),
        global_seq=0,
        gas_price=1,
        chain_id=777,
        mode="cosmos",
        num_txs=1,
        sender_strategy="reuse",
        msg_version="1.4",
        evm_denom="basetcro",
    )

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module.web3, "Web3", lambda _provider: SimpleNamespace(eth=FakeEth())
    )
    monkeypatch.setattr(cli_module, "gen_account", lambda _seq, _index: FakeAccount())
    monkeypatch.setattr(cli_module, "build_cosmos_tx", lambda *args, **kwargs: "raw")
    monkeypatch.setattr(
        cli_module.requests,
        "post",
        lambda url, json: SimpleNamespace(
            json=lambda: {"result": {"code": 5, "log": "insufficient funds"}}
        ),
    )

    result = CliRunner().invoke(
        cli_module.cli,
        [
            "fund",
            "--config",
            "unused.yaml",
            "--mode",
            "cosmos",
            "--batch-size",
            "2",
            "1",
            "2",
        ],
    )

    assert result.exit_code != 0
    assert "insufficient funds" in result.output


def _cosmos_bench_cfg(**overrides):
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm", node_exporter=None),
        endpoints=_ONE_ENDPOINT,
        rpcs=["http://node0"],
        global_seq=0,
        num_txs=1,
        sender_strategy="reuse",
        tx_type="simple-transfer",
        mix_weights=None,
        batch_size=100,
        msg_version="1.4",
        gas_price=1,
        chain_id=777,
        evm_denom="basetcro",
        mode="cosmos",
        send_batch_size=3,
        send_interval=0,
        telemetry=None,
    )
    for key, value in overrides.items():
        setattr(cfg, key, value)
    return cfg


def _mock_cosmos_bench_flow(monkeypatch, cfg):
    """Stub out everything `bench` touches beyond the gates under test."""
    heights = iter([171, 173, 174, 175])
    loaded_txs = {174: ["tx-1"], 175: ["tx-2", "tx-3"]}

    async def fake_send(*_args, **_kwargs):
        return 0

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "gen", lambda *_args, **_kwargs: ["tx-1", "tx-2", "tx-3"])
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "block_height", lambda _rpc: next(heights))
    monkeypatch.setattr(
        cli_module, "block_txs", lambda height, _rpc: loaded_txs.get(height, []), raising=False
    )
    monkeypatch.setattr(cli_module, "MempoolMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "BlockSTMMonitor", FakeMonitor)
    monkeypatch.setattr(cli_module, "_fetch_prometheus", lambda _url: "")
    monkeypatch.setattr(cli_module, "scrape_consensus_raw", lambda _text: {})
    monkeypatch.setattr(
        cli_module, "dump_block_stats", lambda fp, **_kwargs: print("block 175 txs=2", file=fp)
    )


def test_bench_loads_txs_from_cache_instead_of_generating(monkeypatch, tmp_path):
    cache_path = tmp_path / "txs.json"
    cache_path.write_text(
        json.dumps({"num_accounts": 3, "num_txs": 1, "txs": ["tx-1", "tx-2", "tx-3"]})
    )
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    gen_calls = []
    monkeypatch.setattr(cli_module, "gen", lambda *a, **k: gen_calls.append(1) or ["unused"])

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--txs-cache", str(cache_path), "1", "3"],
    )

    assert result.exit_code == 0, result.exception
    assert gen_calls == []
    assert f"loaded 3 cached cosmos txs from {cache_path}" in result.output
    assert "committed_cosmos_txs 3/3" in result.output


def test_bench_writes_generated_txs_to_an_empty_cache_path(monkeypatch, tmp_path):
    cache_path = tmp_path / "txs.json"
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--nonce", "0", "--txs-cache", str(cache_path), "1", "3"],
    )

    assert result.exit_code == 0, result.exception
    written = json.loads(cache_path.read_text())
    assert written == {"num_accounts": 3, "num_txs": 1, "txs": ["tx-1", "tx-2", "tx-3"]}
    # write-then-rename must not leave a stray tmp file behind
    assert list(tmp_path.iterdir()) == [cache_path]


def test_bench_rejects_a_txs_cache_generated_for_a_different_account_count(monkeypatch, tmp_path):
    cache_path = tmp_path / "txs.json"
    cache_path.write_text(json.dumps({"num_accounts": 99, "num_txs": 1, "txs": ["tx-1"]}))
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--txs-cache", str(cache_path), "1", "3"],
    )

    assert result.exit_code != 0
    assert "was generated for 99 accounts x 1 txs, but this run covers 3 accounts x 1 txs" in result.output


def test_bench_rejects_a_txs_cache_generated_for_a_different_num_txs(monkeypatch, tmp_path):
    cache_path = tmp_path / "txs.json"
    cache_path.write_text(json.dumps({"num_accounts": 3, "num_txs": 5, "txs": ["tx-1"]}))
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)

    result = CliRunner().invoke(
        cli_module.cli,
        ["bench", "--config", "unused.yaml", "--txs-cache", str(cache_path), "1", "3"],
    )

    assert result.exit_code != 0
    assert "was generated for 3 accounts x 5 txs, but this run covers 3 accounts x 1 txs" in result.output


def test_bench_exits_non_zero_on_app_hash_divergence(monkeypatch):
    # A divergence check that nothing gates on is a check that doesn't exist:
    # every tx committed, so without the gate this run exits 0.
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    monkeypatch.setattr(
        cli_module,
        "check_divergence",
        lambda _endpoints: {
            "heights": {"node0": 175, "node1": 175},
            "height_skew": 0,
            "app_hash_divergences": [
                {"height": 174, "reason": "app_hash divergence at height 174"}
            ],
        },
    )

    result = CliRunner().invoke(
        cli_module.cli, ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"]
    )

    assert result.exit_code != 0
    assert "state divergence detected" in result.output
    assert "app_hash divergence at height 174" in result.output


def test_bench_exits_non_zero_when_a_node_is_thousands_of_blocks_behind(monkeypatch):
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    monkeypatch.setattr(
        cli_module,
        "check_divergence",
        lambda _endpoints: {
            "heights": {"node0": 5175, "node1": 175},
            "height_skew": 5000,
            "app_hash_divergences": [],
        },
    )

    result = CliRunner().invoke(
        cli_module.cli, ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"]
    )

    assert result.exit_code != 0
    assert "height skew 5000 blocks" in result.output


def test_bench_passes_when_nodes_agree(monkeypatch):
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    monkeypatch.setattr(
        cli_module,
        "check_divergence",
        lambda _endpoints: {
            "heights": {"node0": 175, "node1": 174},
            "height_skew": 1,
            "app_hash_divergences": [],
        },
    )

    result = CliRunner().invoke(
        cli_module.cli, ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"]
    )

    assert result.exit_code == 0, result.exception


def _sweep_run(summary):
    return {
        "mode": "cosmos", "load_start": 1, "load_end": 2, "stats_text": "",
        "summary": summary, "committed_txs": 1, "expected_txs": 1,
    }


def test_sweep_exits_non_zero_when_a_cell_does_not_commit_all_its_txs(monkeypatch, tmp_path):
    # Same hard failure `bench` applies: a cell that measured a truncated load
    # window used to be reported as a passing cell.
    cfg = SimpleNamespace(mode="cosmos", endpoints=_ONE_ENDPOINT)
    matrix_path = tmp_path / "matrix.json"
    matrix_path.write_text('{"axes": {"workers": [8]}}')

    run = _sweep_run({"total_counted_txs": 100, "total_failed_txs": 0, "gas_utilizations": [0.95]})
    run["committed_txs"] = 7
    run["expected_txs"] = 10

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "_run_bench_once", lambda *_args, **_kwargs: run)
    monkeypatch.setattr(cli_module, "build_run_record", lambda **_kwargs: {})
    monkeypatch.setattr(cli_module, "write_run_record", lambda *_args: None)

    result = CliRunner().invoke(
        cli_module.cli,
        ["sweep", "--config", "unused.yaml", "--results-dir", str(tmp_path / "out"),
         str(matrix_path), "1", "3"],
    )

    assert result.exit_code != 0
    assert "timed out waiting for generated transactions to commit" in result.output
    assert "workers8: 7/10" in result.output


def test_bench_exits_non_zero_on_byzantine_validators(monkeypatch):
    # scrape_consensus_health used to be printed and nothing else: a run with a
    # byzantine validator exited 0.
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    monkeypatch.setattr(
        cli_module,
        "dump_block_stats",
        lambda fp, **_kwargs: {"byzantine_validators": 1.0, "missing_validators": 0.0},
    )

    result = CliRunner().invoke(
        cli_module.cli, ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"]
    )

    assert result.exit_code != 0
    assert "1 byzantine validator(s)" in result.output


def test_bench_warns_but_passes_on_missing_validators(monkeypatch):
    # missing_validators is a gauge for the single sampled block, so one missed
    # precommit under saturation load — or a deliberately tiny-stake validator
    # that is expected to be offline — must not abort the run.
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    monkeypatch.setattr(
        cli_module,
        "dump_block_stats",
        lambda fp, **_kwargs: {"byzantine_validators": 0.0, "missing_validators": 1.0},
    )

    result = CliRunner().invoke(
        cli_module.cli, ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"]
    )

    assert result.exit_code == 0, result.exception
    assert "consensus health: run 1: 1 missing validator(s)" in result.output
    assert "state divergence detected" not in result.output


def test_bench_warns_but_passes_when_divergence_could_not_be_verified(monkeypatch):
    # An unreachable node never established a mismatch; aborting on it would fail
    # runs whose only problem was a slow or dead peer.
    cfg = _cosmos_bench_cfg()
    _mock_cosmos_bench_flow(monkeypatch, cfg)
    monkeypatch.setattr(
        cli_module,
        "check_divergence",
        lambda _endpoints: {
            "heights": {"node0": 175, "node1": None},
            "height_skew": None,
            "app_hash_divergences": [
                {"kind": "unverified", "reason": "no committed app hash from ['node1']"}
            ],
        },
    )

    result = CliRunner().invoke(
        cli_module.cli, ["bench", "--config", "unused.yaml", "--nonce", "0", "1", "3"]
    )

    assert result.exit_code == 0, result.exception
    assert "divergence check unverified" in result.output
    assert "state divergence detected" not in result.output


def test_sweep_exits_non_zero_when_the_last_cell_fails_saturation(monkeypatch, tmp_path):
    # The last cell failing leaves ran == total, so the "stopped after N/M"
    # warning never fires and the sweep used to exit 0.
    cfg = SimpleNamespace(mode="cosmos", endpoints=_ONE_ENDPOINT)
    matrix_path = tmp_path / "matrix.json"
    matrix_path.write_text('{"axes": {"workers": [8, 16]}}')

    summaries = iter(
        [
            {"total_counted_txs": 100, "total_failed_txs": 0, "gas_utilizations": [0.95]},
            {"total_counted_txs": 100, "total_failed_txs": 0, "gas_utilizations": [0.40]},
        ]
    )
    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module,
        "_run_bench_once",
        lambda *_args, **_kwargs: _sweep_run(next(summaries)),
    )
    monkeypatch.setattr(cli_module, "build_run_record", lambda **_kwargs: {})
    monkeypatch.setattr(cli_module, "write_run_record", lambda *_args: None)

    result = CliRunner().invoke(
        cli_module.cli,
        ["sweep", "--config", "unused.yaml", "--results-dir", str(tmp_path / "out"),
         str(matrix_path), "1", "3"],
    )

    assert result.exit_code != 0
    assert "saturation gates not met in 1/2 run cells" in result.output
    assert "workers=16" in result.output
    # the summary report is still written before the failure
    assert (tmp_path / "out" / "sweep-summary.txt").exists()


def test_sweep_exits_non_zero_on_divergence_in_a_cell(monkeypatch, tmp_path):
    cfg = SimpleNamespace(mode="cosmos", endpoints=_ONE_ENDPOINT)
    matrix_path = tmp_path / "matrix.json"
    matrix_path.write_text('{"axes": {"workers": [8]}}')

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(
        cli_module,
        "_run_bench_once",
        lambda *_args, **_kwargs: _sweep_run(
            {"total_counted_txs": 100, "total_failed_txs": 0, "gas_utilizations": [0.95]}
        ),
    )
    monkeypatch.setattr(cli_module, "build_run_record", lambda **_kwargs: {})
    monkeypatch.setattr(cli_module, "write_run_record", lambda *_args: None)
    monkeypatch.setattr(
        cli_module,
        "check_divergence",
        lambda _endpoints: {
            "heights": {},
            "height_skew": 0,
            "app_hash_divergences": [{"reason": "app_hash divergence at height 9"}],
        },
    )

    result = CliRunner().invoke(
        cli_module.cli,
        ["sweep", "--config", "unused.yaml", "--results-dir", str(tmp_path / "out"),
         str(matrix_path), "1", "3"],
    )

    assert result.exit_code != 0
    assert "state divergence detected" in result.output
    assert "workers8: app_hash divergence at height 9" in result.output


class _HealthySampler:
    """Checkpoints that pass every soak gate, so only the gate under test can
    fail the command."""

    def __init__(self, *_args):
        self.checkpoints = [
            {"elapsed_s": t, "height": 100 + t, "tps": 100.0, "avg_block_time_ms": 500.0, "rss_bytes": 1_000}
            for t in (0, 30, 60)
        ]

    def start(self):
        pass

    def stop(self):
        pass


def test_soak_exits_non_zero_on_app_hash_divergence(monkeypatch, tmp_path):
    cfg = _cosmos_bench_cfg()

    async def fake_send(*_args, **_kwargs):
        return 0

    monkeypatch.setattr(cli_module, "load_config", lambda _path: cfg)
    monkeypatch.setattr(cli_module, "gen", lambda *_args, **_kwargs: ["tx"] * 200)
    monkeypatch.setattr(cli_module, "send_round_robin", fake_send)
    monkeypatch.setattr(cli_module, "CheckpointSampler", _HealthySampler)
    _stub_soak_wait(monkeypatch)
    monkeypatch.setattr(
        cli_module,
        "check_divergence",
        lambda _endpoints: {
            "heights": {"node0": 160, "node1": 160},
            "height_skew": 0,
            "app_hash_divergences": [{"reason": "app_hash divergence at height 150"}],
        },
    )
    results_path = tmp_path / "soak.json"

    result = CliRunner().invoke(
        cli_module.cli,
        ["soak", "--config", "unused.yaml", "--nonce", "0", "--rate", "100",
         "--duration", "60", "--results", str(results_path), "1", "100"],
    )

    assert result.exit_code != 0
    assert "state divergence detected" in result.output
    # the record is written before the gate raises, and carries the check
    assert "app_hash divergence at height 150" in results_path.read_text()


def _preflight_cfg(endpoints):
    return SimpleNamespace(endpoints=endpoints)


def _stub_probe(monkeypatch, node_ids, peer_ids):
    monkeypatch.setattr(
        cli_module, "probe_peers", lambda _endpoints: (node_ids, peer_ids)
    )


def test_preflight_fails_when_nodes_declare_different_mempool_types(monkeypatch):
    # Numbers measured across two different mempools describe neither config, so
    # a real disagreement can't be a warning that still exits 0.
    endpoints = [
        SimpleNamespace(name="node0", rpc="http://a", node_config={"mempool.type": "app"}),
        SimpleNamespace(name="node1", rpc="http://b", node_config={"mempool.type": "flood"}),
    ]
    monkeypatch.setattr(cli_module, "load_config", lambda _path: _preflight_cfg(endpoints))
    _stub_probe(monkeypatch, {"node0": "id-a", "node1": "id-b"}, {"node0": {"id-b"}, "node1": {"id-a"}})

    result = CliRunner().invoke(cli_module.cli, ["preflight", "--config", "unused.yaml"])

    assert result.exit_code != 0
    assert "disagree on mempool.type" in result.output


def test_preflight_passes_when_mempool_type_is_undeclared_everywhere(monkeypatch):
    endpoints = [
        SimpleNamespace(name="node0", rpc="http://a", node_config={}),
        SimpleNamespace(name="node1", rpc="http://b", node_config={}),
    ]
    monkeypatch.setattr(cli_module, "load_config", lambda _path: _preflight_cfg(endpoints))
    _stub_probe(monkeypatch, {"node0": "id-a", "node1": "id-b"}, {"node0": {"id-b"}, "node1": {"id-a"}})

    result = CliRunner().invoke(cli_module.cli, ["preflight", "--config", "unused.yaml"])

    assert result.exit_code == 0, result.exception
    assert "(undeclared)" in result.output


def test_preflight_fails_on_a_single_unreachable_node(monkeypatch):
    # Two endpoints leave the live node's matrix row all-None as well, so the old
    # all-None-row heuristic couldn't tell a dead node from a healthy pair.
    endpoints = [
        SimpleNamespace(name="node0", rpc="http://a", node_config={}),
        SimpleNamespace(name="node1", rpc="http://b", node_config={}),
    ]
    monkeypatch.setattr(cli_module, "load_config", lambda _path: _preflight_cfg(endpoints))
    _stub_probe(monkeypatch, {"node0": "id-a", "node1": None}, {"node0": set(), "node1": None})

    result = CliRunner().invoke(cli_module.cli, ["preflight", "--config", "unused.yaml"])

    assert result.exit_code != 0
    assert "unreachable nodes: ['node1']" in result.output


def test_preflight_fails_when_the_only_node_is_unreachable(monkeypatch):
    endpoints = [SimpleNamespace(name="node0", rpc="http://a", node_config={})]
    monkeypatch.setattr(cli_module, "load_config", lambda _path: _preflight_cfg(endpoints))
    _stub_probe(monkeypatch, {"node0": None}, {"node0": None})

    result = CliRunner().invoke(cli_module.cli, ["preflight", "--config", "unused.yaml"])

    assert result.exit_code != 0
    assert "unreachable nodes: ['node0']" in result.output


def test_query_account_retries_the_stale_latest_height_race(monkeypatch):
    monkeypatch.setattr(cli_module.time, "sleep", lambda _seconds: None)
    calls = []

    class FakeEth:
        def get_transaction_count(self, _addr):
            calls.append(1)
            if len(calls) == 1:
                raise ValueError("failed to load state at height 98")
            return 5

        def get_balance(self, _addr):
            return 100

    nonce, balance = cli_module._query_account(SimpleNamespace(eth=FakeEth()), "0xabc")

    assert (nonce, balance) == (5, 100)
    assert len(calls) == 2


def test_query_account_gives_up_immediately_on_an_unrelated_value_error(monkeypatch):
    monkeypatch.setattr(cli_module.time, "sleep", lambda _seconds: None)

    class FakeEth:
        def get_transaction_count(self, _addr):
            raise ValueError("boom")

    try:
        cli_module._query_account(SimpleNamespace(eth=FakeEth()), "0xabc")
        assert False, "expected ValueError to propagate"
    except ValueError as e:
        assert str(e) == "boom"
