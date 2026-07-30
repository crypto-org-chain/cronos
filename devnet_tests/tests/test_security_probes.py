from types import SimpleNamespace

from devnet_tests.security_probes import BridgeRejectionResult, send_unauthorized_cro_bridge_call

ADDRESS = "0x" + "1" * 40
DEPLOYED_ADDRESS = "0x" + "2" * 40


class _FakeAccount:
    def __init__(self):
        self.address = ADDRESS
        self.signed = []

    def sign_transaction(self, tx):
        self.signed.append(dict(tx))
        return SimpleNamespace(raw_transaction=b"raw")


class _FakeBuildable:
    def __init__(self, base_tx):
        self._base_tx = base_tx

    def build_transaction(self, overrides):
        return {**self._base_tx, **overrides}


class _FakeContract:
    def __init__(self, address=None):
        self.address = address
        self.functions = SimpleNamespace(
            send_cro_to_crypto_org=lambda recipient: _FakeBuildable({"recipient": recipient})
        )

    def constructor(self):
        return _FakeBuildable({"deploy": True})


def _receipts(*statuses):
    remaining = iter(statuses)

    def wait_for_transaction_receipt(tx_hash):
        return SimpleNamespace(contractAddress=DEPLOYED_ADDRESS, status=next(remaining))

    return wait_for_transaction_receipt


def _fake_w3(
    send_raw_transaction=lambda raw: "0xhash",
    wait_for_transaction_receipt=None,
    start_nonce=5,
):
    # Sentinel rather than a default _receipts(1, 1) so each _fake_w3 gets its
    # own receipt iterator instead of sharing one captured at import time.
    eth = SimpleNamespace(
        chain_id=777,
        gas_price=1000,
        get_transaction_count=lambda addr: start_nonce,
        send_raw_transaction=send_raw_transaction,
        wait_for_transaction_receipt=wait_for_transaction_receipt or _receipts(1, 1),
        contract=lambda **kwargs: _FakeContract(address=kwargs.get("address")),
    )
    return SimpleNamespace(eth=eth)


def test_unauthorized_call_reverted_is_reported_as_rejected():
    account = _FakeAccount()
    w3 = _fake_w3(wait_for_transaction_receipt=_receipts(1, 0))
    result = send_unauthorized_cro_bridge_call(w3, account)
    assert result == BridgeRejectionResult(rejected=True)


def test_unexpectedly_successful_call_is_reported_as_not_rejected():
    account = _FakeAccount()
    w3 = _fake_w3(wait_for_transaction_receipt=_receipts(1, 1))
    result = send_unauthorized_cro_bridge_call(w3, account)
    assert result == BridgeRejectionResult(rejected=False)


def test_deploy_failure_is_captured_without_raising():
    account = _FakeAccount()

    def raising(raw):
        raise RuntimeError("insufficient funds")

    w3 = _fake_w3(send_raw_transaction=raising)
    result = send_unauthorized_cro_bridge_call(w3, account)
    assert result == BridgeRejectionResult(rejected=False, error="insufficient funds")


def test_deploy_receipt_with_failed_status_is_reported_as_error():
    account = _FakeAccount()
    w3 = _fake_w3(wait_for_transaction_receipt=_receipts(0))
    result = send_unauthorized_cro_bridge_call(w3, account)
    assert result == BridgeRejectionResult(rejected=False, error="CroBridge deployment failed")


def test_call_submission_failure_is_captured_without_raising():
    account = _FakeAccount()
    calls = []

    def send_raw_transaction(raw):
        calls.append(raw)
        if len(calls) == 2:
            raise RuntimeError("account sequence mismatch")
        return "0xhash"

    w3 = _fake_w3(send_raw_transaction=send_raw_transaction)
    result = send_unauthorized_cro_bridge_call(w3, account)
    assert result == BridgeRejectionResult(rejected=False, error="account sequence mismatch")


def test_deploy_and_call_use_consecutive_nonces():
    account = _FakeAccount()
    w3 = _fake_w3(start_nonce=10)
    send_unauthorized_cro_bridge_call(w3, account)
    nonces = [tx["nonce"] for tx in account.signed]
    assert nonces == [10, 11]
