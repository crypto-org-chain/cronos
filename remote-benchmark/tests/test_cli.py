from types import SimpleNamespace

from click.testing import CliRunner

from remote_benchmark import cli as cli_module


class FakeMonitor:
    def __init__(self, *_args):
        self.data = {}

    def start(self):
        pass

    def stop(self):
        pass


def test_bench_waits_for_all_generated_txs_to_commit(monkeypatch):
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm"),
        rpcs=["http://node0", "http://node1", "http://node2"],
        global_seq=0,
        num_txs=1,
        tx_type="simple-transfer",
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
        pass

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
        ["bench", "--config", "unused.yaml", "1", "3"],
    )

    assert result.exit_code == 0, result.exception
    assert "block 175 txs=2" in result.output
    assert "committed_cosmos_txs 3/3" in result.output
    assert "no_load_period" not in result.output


def test_bench_fails_when_not_all_generated_txs_commit(monkeypatch):
    cfg = SimpleNamespace(
        primary=SimpleNamespace(rpc="http://node0", json_rpc="http://node0-evm"),
        rpcs=["http://node0"],
        global_seq=0,
        num_txs=1,
        tx_type="simple-transfer",
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
        pass

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
        ["bench", "--config", "unused.yaml", "1", "2"],
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
    assert posts[0][0] == "http://cosmos-rpc"
