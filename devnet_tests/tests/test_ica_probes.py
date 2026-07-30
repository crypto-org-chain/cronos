from types import SimpleNamespace

from devnet_tests.ica_probes import IcaRejectionResult, register_ica_with_unknown_connection

ADDRESS = "0x" + "1" * 40


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


class _FakeIcaContract:
    def __init__(self):
        self.functions = SimpleNamespace(
            registerAccount=lambda connection_id, version, ordering: _FakeBuildable(
                {"connectionID": connection_id, "version": version, "ordering": ordering}
            )
        )


def _receipt(status):
    return lambda tx_hash: SimpleNamespace(status=status)


def _fake_w3(
    send_raw_transaction=lambda raw: "0xhash",
    wait_for_transaction_receipt=_receipt(0),
    start_nonce=5,
):
    eth = SimpleNamespace(
        chain_id=777,
        gas_price=1000,
        get_transaction_count=lambda addr: start_nonce,
        send_raw_transaction=send_raw_transaction,
        wait_for_transaction_receipt=wait_for_transaction_receipt,
        contract=lambda **kwargs: _FakeIcaContract(),
    )
    return SimpleNamespace(eth=eth)


def test_unknown_connection_reverted_is_reported_as_rejected():
    account = _FakeAccount()
    w3 = _fake_w3(wait_for_transaction_receipt=_receipt(0))
    result = register_ica_with_unknown_connection(w3, account)
    assert result == IcaRejectionResult(rejected=True)


def test_unexpectedly_successful_call_is_reported_as_not_rejected():
    account = _FakeAccount()
    w3 = _fake_w3(wait_for_transaction_receipt=_receipt(1))
    result = register_ica_with_unknown_connection(w3, account)
    assert result == IcaRejectionResult(rejected=False)


def test_submission_failure_is_captured_without_raising():
    account = _FakeAccount()

    def raising(raw):
        raise RuntimeError("insufficient funds")

    w3 = _fake_w3(send_raw_transaction=raising)
    result = register_ica_with_unknown_connection(w3, account)
    assert result == IcaRejectionResult(rejected=False, error="insufficient funds")


def test_call_sets_an_explicit_gas_value():
    # A missing "gas" key makes real web3.py estimate gas by simulating the
    # call, which raises on a revert before the tx is ever sent — this guards
    # against reintroducing that (see ica_probes.py's comment on the tx dict).
    account = _FakeAccount()
    w3 = _fake_w3()
    register_ica_with_unknown_connection(w3, account)
    assert account.signed[0]["gas"] > 0
