from types import SimpleNamespace

from remote_benchmark import runner as runner_module


def test_wait_for_committed_returns_the_height_that_actually_hit_the_threshold():
    # The chain runs ahead to height 205 (get_height) before the block that
    # actually hits expected_txs (202) gets counted. Returning that stale,
    # further-extended `end` instead of the height just counted would make
    # callers re-scan blocks 203-205 for stats, over-counting txs that were
    # never part of this load.
    per_block = {201: 2, 202: 3, 203: 1, 204: 1, 205: 1}
    end, committed = runner_module._wait_for_committed(
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
        runner_module.web3,
        "Web3",
        lambda _provider: SimpleNamespace(eth=FakeEth()),
    )
    monkeypatch.setattr(
        runner_module,
        "gen_account",
        lambda _seq, index: SimpleNamespace(address=f"account-{index}"),
    )

    try:
        runner_module.current_sender_nonce(cfg, 3, 3)
    except ValueError as exc:
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
        runner_module.web3,
        "Web3",
        lambda _provider: SimpleNamespace(
            eth=SimpleNamespace(
                get_transaction_count=lambda address: requested_addresses.append(address) or 0
            )
        ),
    )
    monkeypatch.setattr(
        runner_module,
        "gen_account",
        lambda _seq, index: SimpleNamespace(address=f"account-{index}"),
    )

    # cfg.num_txs=1 would only check account-0; the soak's own num_txs=4 is what
    # gen() actually signs from.
    assert runner_module.current_sender_nonce(cfg, 0, 0, num_txs=4) == 0
    assert requested_addresses == ["account-0", "account-1", "account-2", "account-3"]
