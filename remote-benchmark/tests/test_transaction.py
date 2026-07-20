from eth_account import Account
from eth_account._utils.legacy_transactions import Transaction
from hexbytes import HexBytes

from remote_benchmark.transaction import gen, physical_account_range
from remote_benchmark.utils import gen_account


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
