import asyncio

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


class ImmediatePool:
    def __enter__(self):
        return self

    def __exit__(self, *_args):
        pass

    def map(self, func, jobs):
        return [func(job) for job in jobs]


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
            [f"tx-{i}" for i in range(4)], "http://node0", batch_size=2, batch_interval=0
        )
    )

    assert [raw for raw, _ in sent] == ["tx-0", "tx-1", "tx-2", "tx-3"]


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
            [f"tx-{i}" for i in range(4)], "http://node0", batch_size=2, batch_interval=0
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
