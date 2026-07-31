from types import SimpleNamespace

from devnet_tests.eip_probes import (
    _MAX_TX_GAS,
    ProbeResult,
    send_below_base_fee,
    send_blob_tx,
    send_insufficient_balance,
    send_over_max_tx_gas,
    send_under_floor_data_gas,
)

ADDRESS = "0x" + "1" * 40


class _FakeAccount:
    def __init__(self, address=ADDRESS):
        self.address = address
        self.calls = []

    def sign_transaction(self, tx, blobs=None):
        self.calls.append((dict(tx), blobs))
        return SimpleNamespace(raw_transaction=b"raw")


def _fake_w3(send_raw_transaction=lambda raw: None, base_fee=1000):
    nonce_lookups = []

    def get_transaction_count(addr, block="latest"):
        nonce_lookups.append((addr, block))
        return 5

    eth = SimpleNamespace(
        chain_id=777,
        gas_price=1000,
        get_transaction_count=get_transaction_count,
        get_block=lambda tag: {"baseFeePerGas": base_fee},
        send_raw_transaction=send_raw_transaction,
    )
    return SimpleNamespace(eth=eth, nonce_lookups=nonce_lookups)


def test_submit_reports_accepted_on_success():
    account = _FakeAccount()
    w3 = _fake_w3()
    result = send_over_max_tx_gas(w3, account)
    assert result == ProbeResult(accepted=True, error=None)
    assert w3.nonce_lookups == [(ADDRESS, "pending")]


def test_submit_captures_exception_as_error():
    def raising(raw):
        raise RuntimeError("boom")

    account = _FakeAccount()
    result = send_over_max_tx_gas(_fake_w3(send_raw_transaction=raising), account)
    assert result == ProbeResult(accepted=False, error="boom")


def test_send_over_max_tx_gas_builds_tx_above_cap():
    account = _FakeAccount()
    send_over_max_tx_gas(_fake_w3(base_fee=1000), account)
    (tx, _blobs), = account.calls
    assert tx["gas"] > _MAX_TX_GAS
    assert tx["maxFeePerGas"] == 1000


def test_send_under_floor_data_gas_stays_between_intrinsic_and_floor():
    account = _FakeAccount()
    send_under_floor_data_gas(_fake_w3(), account)
    (tx, _blobs), = account.calls
    nonzero_bytes = len(tx["data"])
    intrinsic = 21000 + nonzero_bytes * 16
    floor = 21000 + nonzero_bytes * 40
    assert intrinsic < tx["gas"] < floor


def test_send_below_base_fee_stays_under_base_fee():
    account = _FakeAccount()
    send_below_base_fee(_fake_w3(base_fee=1000), account)
    (tx, _blobs), = account.calls
    assert tx["maxFeePerGas"] < 1000


def test_send_insufficient_balance_uses_a_fresh_unfunded_account(monkeypatch):
    unfunded = _FakeAccount(address="0x" + "2" * 40)
    monkeypatch.setattr(
        "devnet_tests.eip_probes.Account.create", staticmethod(lambda: unfunded)
    )

    account = _FakeAccount()
    w3 = _fake_w3()
    send_insufficient_balance(w3, account)

    assert account.calls == []
    (tx, _blobs), = unfunded.calls
    assert tx["to"] == account.address
    assert tx["value"] == 1
    assert tx["nonce"] == 0
    # The probe pins nonce 0 itself, so it must not query the funded account's.
    assert w3.nonce_lookups == []


def test_send_blob_tx_passes_a_blob():
    account = _FakeAccount()
    send_blob_tx(_fake_w3(), account)
    (_tx, blobs), = account.calls
    assert blobs and len(blobs) == 1
