import asyncio

import pytest
from eth_account import Account
from eth_account._utils.legacy_transactions import Transaction
from hexbytes import HexBytes

from remote_benchmark import transaction as tx_module
from remote_benchmark.contracts import NFT_ADDRESS, POOL_ADDRESS
from remote_benchmark.erc20 import CONTRACT_ADDRESS
from remote_benchmark.transaction import (
    ERC20_TRANSFER_SELECTOR,
    HOT_RECEIVER_ADDRESS,
    MINT_SELECTOR,
    SWAP_SELECTOR,
    TX_TYPES,
    erc20_transfer_hot_tx,
    gen,
    nft_mint_tx,
    physical_account_range,
    uniswap_swap_tx,
    weighted_mix_tx,
)
from remote_benchmark.utils import gen_account

SENDER = "0x1111111111111111111111111111111111111111"


class ImmediateResult:
    def __init__(self, value):
        self._value = value

    def ready(self):
        return True

    def wait(self, _timeout=None):
        pass

    def get(self):
        return self._value


class ImmediatePool:
    def __init__(self, processes=None, initializer=None, initargs=()):
        if initializer is not None:
            initializer(*initargs)

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        pass

    def map(self, func, jobs):
        return [func(job) for job in jobs]

    def map_async(self, func, jobs):
        return ImmediateResult([func(job) for job in jobs])


def recover_sender(raw):
    return Account.recover_transaction(HexBytes(raw))


def recover_nonce(raw):
    return Transaction.from_bytes(HexBytes(raw)).nonce


def use_immediate_pool(monkeypatch):
    monkeypatch.setattr("remote_benchmark.transaction.os.cpu_count", lambda: 1)
    monkeypatch.setattr(
        "remote_benchmark.transaction.multiprocessing.Pool", ImmediatePool
    )


def test_reuse_sender_strategy_reuses_accounts_and_increments_nonces(monkeypatch):
    use_immediate_pool(monkeypatch)

    txs = gen(
        0,
        num_accounts=2,
        num_txs=2,
        tx_type="simple-transfer",
        batch=1,
        start_account=1,
        wire_format="eth",
        sender_strategy="reuse",
    )

    assert [recover_sender(raw) for raw in txs] == [
        gen_account(0, 1).address,
        gen_account(0, 2).address,
        gen_account(0, 1).address,
        gen_account(0, 2).address,
    ]
    assert [recover_nonce(raw) for raw in txs] == [0, 0, 1, 1]


def test_unique_per_tx_strategy_uses_one_sender_for_each_transaction(monkeypatch):
    use_immediate_pool(monkeypatch)

    txs = gen(
        0,
        num_accounts=2,
        num_txs=2,
        tx_type="simple-transfer",
        batch=1,
        start_account=1,
        wire_format="eth",
        sender_strategy="unique-per-tx",
    )

    assert [recover_sender(raw) for raw in txs] == [
        gen_account(0, 1).address,
        gen_account(0, 3).address,
        gen_account(0, 2).address,
        gen_account(0, 4).address,
    ]
    assert [recover_nonce(raw) for raw in txs] == [0, 0, 0, 0]


def test_physical_account_range_expands_only_unique_per_tx_strategy():
    assert physical_account_range(1, 2, 3, "reuse") == (1, 2)
    assert physical_account_range(1, 2, 3, "unique-per-tx") == (1, 6)


def test_erc20_transfer_hot_tx_always_sends_to_the_fixed_hot_receiver():
    tx = erc20_transfer_hot_tx(SENDER, 0, {})

    assert tx["to"] == CONTRACT_ADDRESS
    assert tx["data"].startswith(ERC20_TRANSFER_SELECTOR)
    assert HOT_RECEIVER_ADDRESS[2:].lower() in tx["data"].lower()
    assert SENDER[2:].lower() not in tx["data"].lower()


def test_uniswap_swap_tx_targets_the_seeded_pool_and_alternates_direction():
    even = uniswap_swap_tx(SENDER, 0, {})
    odd = uniswap_swap_tx(SENDER, 1, {})

    assert even["to"] == POOL_ADDRESS == odd["to"]
    assert even["data"][:10] == SWAP_SELECTOR == odd["data"][:10]
    assert even["data"] != odd["data"]  # zero_for_one flips with nonce parity


def test_nft_mint_tx_targets_the_shared_counter_with_no_args():
    tx = nft_mint_tx(SENDER, 0, {})

    assert tx["to"] == NFT_ADDRESS
    assert tx["data"] == MINT_SELECTOR


def test_weighted_mix_tx_is_deterministic_and_only_dispatches_configured_types():
    mix = {"erc20-transfer-hot": 0.5, "uniswap-swap": 0.3, "nft-mint": 0.2}
    options = {"mix": mix}

    first = weighted_mix_tx(SENDER, 7, options)
    second = weighted_mix_tx(SENDER, 7, options)
    assert first == second  # same (sender, nonce) always picks the same builder

    destinations = {POOL_ADDRESS, NFT_ADDRESS, CONTRACT_ADDRESS}
    for nonce in range(50):
        tx = weighted_mix_tx(f"0xsender{nonce}", nonce, options)
        assert tx["to"] in destinations


def test_weighted_mix_tx_frequencies_roughly_match_configured_weights():
    mix = {"erc20-transfer-hot": 0.5, "uniswap-swap": 0.3, "nft-mint": 0.2}
    options = {"mix": mix}
    to_name = {
        CONTRACT_ADDRESS: "erc20-transfer-hot",
        POOL_ADDRESS: "uniswap-swap",
        NFT_ADDRESS: "nft-mint",
    }
    counts = dict.fromkeys(mix, 0)

    n = 5000
    for i in range(n):
        tx = weighted_mix_tx(f"0xacct{i}", i, options)
        counts[to_name[tx["to"]]] += 1

    for name, weight in mix.items():
        assert abs(counts[name] / n - weight) < 0.03


def test_weighted_mix_is_registered_under_tx_types():
    assert TX_TYPES["weighted-mix"] is weighted_mix_tx


def _capture_sends(monkeypatch):
    sent = []

    async def fake_sendtx(_session, raw, rpc, _sync, _mode):
        sent.append((raw, rpc))
        return True

    monkeypatch.setattr(tx_module, "async_sendtx", fake_sendtx)
    return sent


def test_send_stops_at_the_deadline(monkeypatch, capsys):
    # An open-loop rate target defines the run by its duration; overrunning it
    # to drain the generated txs benchmarks a longer window than requested.
    sent = _capture_sends(monkeypatch)

    asyncio.run(
        tx_module.send(
            [f"tx-{i}" for i in range(10)],
            "http://node0",
            batch_size=2,
            batch_interval=0,
            deadline_s=0,
        )
    )

    assert sent == []
    assert "10/10 txs unsent" in capsys.readouterr().err


def test_send_without_a_deadline_sends_everything(monkeypatch):
    sent = _capture_sends(monkeypatch)

    asyncio.run(
        tx_module.send(
            [f"tx-{i}" for i in range(4)],
            "http://node0",
            batch_size=2,
            batch_interval=0,
        )
    )

    assert [raw for raw, _ in sent] == ["tx-0", "tx-1", "tx-2", "tx-3"]


def test_send_serializes_same_sender_sends_and_forces_sync(monkeypatch):
    # Same-sender nonces race for the mempool's admission lock if a later one
    # is issued before the earlier one's CheckTx has actually completed, and
    # broadcast_tx_async's response can't prove that - so a sender's second
    # send must wait for the first to actually finish, and must go out via
    # broadcast_tx_sync (the only response that reflects real completion)
    # even though the run overall requested async.
    events = []

    async def fake_sendtx(_session, raw, _rpc, sync, _mode):
        events.append((raw, sync, "start"))
        await asyncio.sleep(0)
        events.append((raw, sync, "end"))
        return True

    monkeypatch.setattr(tx_module, "async_sendtx", fake_sendtx)

    asyncio.run(
        tx_module.send(
            [f"tx-{i}" for i in range(4)],
            "http://node0",
            sync=False,
            batch_size=4,
            batch_interval=0,
            probe_batches=0,
            num_accounts=2,
        )
    )

    # tx-0/tx-2 are sender 0's two nonces (position % num_accounts).
    assert ("tx-0", False, "start") in events
    assert ("tx-2", True, "start") in events
    assert events.index(("tx-2", True, "start")) > events.index(("tx-0", False, "end"))


def test_send_round_robin_routes_each_account_to_one_endpoint(monkeypatch):
    # gen() interleaves accounts, so position p belongs to account
    # p % num_accounts; every tx of one account must land on the same node or a
    # later nonce can arrive before an earlier one propagates.
    sent = _capture_sends(monkeypatch)

    asyncio.run(
        tx_module.send_round_robin(
            [f"tx-{i}" for i in range(12)],
            ["http://node0", "http://node1"],
            batch_size=5,
            batch_interval=0,
            num_accounts=3,
        )
    )

    assert len(sent) == 12
    routing = {}
    for raw, rpc in sent:
        account = int(raw.removeprefix("tx-")) % 3
        routing.setdefault(account, set()).add(rpc)
    assert routing == {
        0: {"http://node0"},
        1: {"http://node1"},
        2: {"http://node0"},
    }


def test_send_round_robin_stops_at_the_deadline(monkeypatch):
    sent = _capture_sends(monkeypatch)

    asyncio.run(
        tx_module.send_round_robin(
            [f"tx-{i}" for i in range(10)],
            ["http://node0", "http://node1"],
            batch_size=2,
            batch_interval=0,
            num_accounts=2,
            deadline_s=0,
        )
    )

    assert sent == []


def test_send_returns_the_count_of_txs_whose_retries_never_succeeded(monkeypatch):
    # A tx that keeps failing until async_sendtx's own backoff gives up must be
    # counted, not silently dropped - the caller uses this to size how many
    # commits to actually wait for.
    async def fake_sendtx(_session, raw, _rpc, _sync, _mode):
        return raw != "tx-1"

    monkeypatch.setattr(tx_module, "async_sendtx", fake_sendtx)

    failed = asyncio.run(
        tx_module.send(
            [f"tx-{i}" for i in range(4)],
            "http://node0",
            batch_size=2,
            batch_interval=0,
        )
    )

    assert failed == 1


def test_send_round_robin_returns_the_count_of_txs_whose_retries_never_succeeded(
    monkeypatch,
):
    async def fake_sendtx(_session, raw, _rpc, _sync, _mode):
        return raw not in ("tx-1", "tx-3")

    monkeypatch.setattr(tx_module, "async_sendtx", fake_sendtx)

    failed = asyncio.run(
        tx_module.send_round_robin(
            [f"tx-{i}" for i in range(4)],
            ["http://node0", "http://node1"],
            batch_size=2,
            batch_interval=0,
            num_accounts=2,
        )
    )

    assert failed == 2


class _FakeResponse:
    def __init__(self, payload):
        self._payload = payload

    async def json(self):
        return self._payload


class _FakePost:
    def __init__(self, payload):
        self._payload = payload

    async def __aenter__(self):
        return _FakeResponse(self._payload)

    async def __aexit__(self, *_exc):
        return False


class _FakeSession:
    def __init__(self, payload):
        self._payload = payload
        self.calls = 0

    def post(self, _rpc, json):  # noqa: A002 - matches aiohttp's signature
        self.calls += 1
        return _FakePost(self._payload)


# Pinned as literals rather than read from DUPLICATE_SEND_MARKERS: sourcing them
# from the constant would silently drop a case if a marker were ever removed.
# These are CometBFT's ErrTxInCache and app-mempool's ErrSeenTx.
@pytest.mark.parametrize("marker", ["already exists in cache", "tx already seen"])
def test_async_sendtx_treats_a_duplicate_send_as_success(marker):
    # Whichever mempool is running, a duplicate rejection means the tx is
    # already in flight. Retrying burns the full 60s backoff and then reports
    # the tx as never sent, so it must short-circuit on the first response.
    session = _FakeSession({"error": {"code": -32603, "data": marker}})

    assert asyncio.run(tx_module.async_sendtx(session, "rawtx", "http://node0"))
    assert session.calls == 1


def test_async_sendtx_returns_retry_on_wrong_sequence():
    # A wrong-sequence rejection means the tx itself is fine and just arrived
    # before an earlier nonce for the same sender - the caller resends it
    # rather than counting it as failed, so this must be distinguishable from
    # a plain rejection (which returns False) via the sentinel RETRY value.
    session = _FakeSession(
        {"result": {"code": 5, "log": "incorrect account sequence; expected 2, got 3"}}
    )

    assert (
        asyncio.run(tx_module.async_sendtx(session, "rawtx", "http://node0", True))
        == tx_module.RETRY
    )


def test_async_sendtx_returns_retry_on_ethermint_invalid_nonce():
    # ethermint's own nonce check (ante/eth.go) raises a differently-worded
    # ErrInvalidSequence than the plain cosmos-sdk sequence error - a batch
    # envelope tx wrapping EVM messages hits this text, and it must retry the
    # same as WRONG_SEQUENCE_MARKER rather than being dropped for good.
    session = _FakeSession(
        {"result": {"code": 5, "log": "invalid nonce; got 100, expected 0: invalid sequence"}}
    )

    assert (
        asyncio.run(tx_module.async_sendtx(session, "rawtx", "http://node0", True))
        == tx_module.RETRY
    )


def test_async_sendtx_does_not_retry_wrong_sequence_in_eth_mode():
    # ethermint EVM txs raise a different, unmatched error for a bad nonce, so
    # eth mode must never match on the cosmos-specific WRONG_SEQUENCE_MARKER
    # text and retry something that will never succeed as-is.
    session = _FakeSession({"error": {"message": "incorrect account sequence"}})

    assert (
        asyncio.run(
            tx_module.async_sendtx(session, "rawtx", "http://node0", mode="eth")
        )
        is False
    )


async def _fake_sleep(_seconds):
    pass


def test_drain_retries_resends_until_it_succeeds(monkeypatch):
    monkeypatch.setattr(tx_module.asyncio, "sleep", _fake_sleep)
    attempts = {"tx-0": 0}

    async def fake_sendtx(_session, raw, _rpc, _sync, _mode):
        attempts[raw] += 1
        return tx_module.RETRY if attempts[raw] < 2 else True

    monkeypatch.setattr(tx_module, "async_sendtx", fake_sendtx)

    failed = asyncio.run(
        tx_module._drain_retries(None, [("tx-0", "http://node0")], "cosmos")
    )

    assert failed == 0
    assert attempts["tx-0"] == 2


def test_drain_retries_gives_up_after_max_rounds(monkeypatch):
    monkeypatch.setattr(tx_module.asyncio, "sleep", _fake_sleep)

    async def always_retry(_session, _raw, _rpc, _sync, _mode):
        return tx_module.RETRY

    monkeypatch.setattr(tx_module, "async_sendtx", always_retry)

    failed = asyncio.run(
        tx_module._drain_retries(
            None,
            [("tx-0", "http://node0"), ("tx-1", "http://node0")],
            "cosmos",
        )
    )

    assert failed == 2


def test_send_multiprocess_falls_back_to_single_process_for_one_worker(monkeypatch):
    # The forced sync=True below is only needed to guard against cross-process
    # CheckTx reordering - with a single worker there's no second process to
    # race against, so this fallback must keep the caller's own sync setting
    # instead of overriding it.
    calls = []

    async def fake_send_round_robin(txs, rpcs, num_accounts, **kwargs):
        calls.append((txs, rpcs, num_accounts, kwargs))
        return 0

    monkeypatch.setattr(tx_module, "send_round_robin", fake_send_round_robin)

    failed = tx_module.send_multiprocess(
        ["tx-0", "tx-1"], ["http://node0"], num_accounts=1, num_workers=1
    )

    assert failed == 0
    assert calls == [(["tx-0", "tx-1"], ["http://node0"], 1, {})]


def test_send_multiprocess_splits_by_account_range_and_overrides_batch_size(
    monkeypatch,
):
    # send_round_robin requires each account's txs to arrive in nonce order,
    # so a worker must own an account's whole nonce sequence, not an
    # arbitrary slice - splitting by ACCOUNT range (not flat position)
    # guarantees that.
    jobs_seen = []

    def fake_send_worker(args):
        txs, rpcs, kwargs = args
        jobs_seen.append((txs, rpcs, kwargs))
        return 0

    monkeypatch.setattr(tx_module.multiprocessing, "Pool", ImmediatePool)
    monkeypatch.setattr(tx_module, "_send_worker", fake_send_worker)

    # 4 accounts x 2 nonce-rounds, laid out as gen() would: interleaved by
    # account within each round.
    txs = [f"acct{a}-nonce{n}" for n in range(2) for a in range(4)]

    failed = tx_module.send_multiprocess(
        txs,
        ["http://node0", "http://node1", "http://node2"],
        num_accounts=4,
        num_workers=2,
    )

    assert failed == 0
    assert len(jobs_seen) == 2

    worker0_txs, worker0_rpcs, worker0_kwargs = jobs_seen[0]
    worker1_txs, worker1_rpcs, worker1_kwargs = jobs_seen[1]

    # accounts 0-1 go to worker 0, accounts 2-3 to worker 1 - each worker's
    # txs stay within its own account range across both nonce rounds.
    assert worker0_txs == ["acct0-nonce0", "acct1-nonce0", "acct0-nonce1", "acct1-nonce1"]
    assert worker1_txs == ["acct2-nonce0", "acct3-nonce0", "acct2-nonce1", "acct3-nonce1"]

    # batch_size is overridden to each worker's own (shrunk) account count,
    # not the global num_accounts, so a worker's batch never spans multiple
    # nonce rounds.
    assert worker0_kwargs["num_accounts"] == worker0_kwargs["batch_size"] == 2
    assert worker1_kwargs["num_accounts"] == worker1_kwargs["batch_size"] == 2
    assert worker0_kwargs["sync"] is True

    # worker 1's account range starts at global index 2, so its rpcs are
    # rotated by 2 - its local account 0 lands on rpcs[2], the same endpoint
    # global account 2 would get in a single-process round robin, instead of
    # every worker's local account 0 clustering onto rpcs[0].
    assert worker0_rpcs == ["http://node0", "http://node1", "http://node2"]
    assert worker1_rpcs == ["http://node2", "http://node0", "http://node1"]
