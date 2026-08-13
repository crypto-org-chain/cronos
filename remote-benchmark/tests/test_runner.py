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
        count_txs_batch=lambda lo, hi: {h: per_block.get(h, 0) for h in range(lo, hi + 1)},
        start=200,
        end=201,
        expected_txs=5,
    )

    assert end == 202
    assert committed == 5


def test_wait_for_committed_txs_batches_the_scan_via_blockchain_range(monkeypatch):
    # blockchain_range can return many heights per call (e.g. a /blockchain
    # page); the loop must stop at the height inside that batch that actually
    # hits the threshold, not consume the whole batch.
    metas = {201: (2, "t201"), 202: (3, "t202"), 203: (1, "t203")}
    calls = []

    def fake_blockchain_range(lo, hi, rpc):
        calls.append((lo, hi))
        return {h: metas[h] for h in range(lo, hi + 1) if h in metas}

    monkeypatch.setattr(runner_module, "blockchain_range", fake_blockchain_range)

    end, committed = runner_module.wait_for_committed_txs(
        "http://rpc", start=200, end=203, expected_txs=5
    )

    assert end == 202
    assert committed == 5
    assert calls == [(201, 203)]


def test_wait_for_committed_eth_txs_scans_the_whole_chunk(monkeypatch):
    # Regression: count_txs_batch(lo, hi) used to fetch only `lo`, silently
    # dropping the rest of the chunk instead of scanning [lo, hi].
    per_block = {201: 2, 202: 3, 203: 1}
    calls = []

    def fake_block_eth(height, json_rpc):
        calls.append(height)
        return {"transactions": [None] * per_block.get(height, 0)}

    monkeypatch.setattr(runner_module, "block_eth", fake_block_eth)

    end, committed = runner_module.wait_for_committed_eth_txs(
        "http://json-rpc", start=200, end=203, expected_txs=5
    )

    assert end == 202
    assert committed == 5
    assert calls == [201, 202, 203]


def test_wait_for_committed_eth_txs_caps_waste_past_threshold_to_a_small_chunk(
    monkeypatch,
):
    # eth has no batch endpoint - each height is its own HTTP call, so a large
    # chunk would eagerly fetch (and pay for) many heights past the point the
    # threshold is already satisfied. A small eth-specific chunk bounds that
    # waste, unlike the Cosmos path where one /blockchain call is cheap however
    # many heights it covers.
    per_block = {201: 5}
    calls = []

    def fake_block_eth(height, json_rpc):
        calls.append(height)
        return {"transactions": [None] * per_block.get(height, 0)}

    monkeypatch.setattr(runner_module, "block_eth", fake_block_eth)

    end, committed = runner_module.wait_for_committed_eth_txs(
        "http://json-rpc", start=200, end=1000, expected_txs=5
    )

    assert end == 201
    assert committed == 5
    # The dict comprehension behind count_txs_batch still fetches every height
    # in the chunk eagerly before the threshold check runs - a small chunk
    # caps that waste to 20 calls instead of the Cosmos-sized 200.
    assert calls == list(range(201, 221))


def test_wait_for_committed_backs_off_on_an_empty_batch(monkeypatch):
    # A batch call that reports zero heights must not spin the loop with no
    # delay - it should sleep before retrying, same as the "caught up to the
    # chain tip" branch already does.
    sleeps = []
    monkeypatch.setattr(runner_module.time, "sleep", lambda s: sleeps.append(s))

    calls = []

    def count_txs_batch(lo, hi):
        calls.append((lo, hi))
        if len(calls) >= 3:
            return {lo: 5}
        return {}

    end, committed = runner_module._wait_for_committed(
        get_height=lambda: 205,
        count_txs_batch=count_txs_batch,
        start=200,
        end=205,
        expected_txs=5,
    )

    assert committed == 5
    assert len(calls) == 3
    assert sleeps == [0.2, 0.2]


def test_wait_for_committed_gives_up_after_commits_stall(monkeypatch):
    # A tx dropped by mempool recheck never arrives - without a stall check
    # this would otherwise wait out the full timeout doing nothing.
    monkeypatch.setattr(runner_module.time, "sleep", lambda s: None)
    per_block = {201: 3}  # commits stop moving after height 201

    end, committed = runner_module._wait_for_committed(
        get_height=lambda: 400,
        count_txs_batch=lambda lo, hi: {h: per_block.get(h, 0) for h in range(lo, hi + 1)},
        start=200,
        end=400,
        expected_txs=10,
        stall_blocks=10,
    )

    assert committed == 3
    assert end == 211  # 10 stalled blocks after the last count change at 201


def test_wait_for_committed_stall_check_ignores_the_zero_commit_ramp_up(monkeypatch):
    # No tx has landed yet while sends are still going out - that's normal
    # ramp-up, not a stall, so it must not trip the stall exit before the
    # first tx actually commits.
    monkeypatch.setattr(runner_module.time, "sleep", lambda s: None)
    per_block = {215: 5}  # first commit lands well past stall_blocks=10

    end, committed = runner_module._wait_for_committed(
        get_height=lambda: 400,
        count_txs_batch=lambda lo, hi: {h: per_block.get(h, 0) for h in range(lo, hi + 1)},
        start=200,
        end=400,
        expected_txs=5,
        stall_blocks=10,
    )

    assert committed == 5
    assert end == 215


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
