from types import SimpleNamespace

from click.testing import CliRunner

from remote_benchmark import cli as cli_module


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
