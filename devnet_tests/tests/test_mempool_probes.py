from types import SimpleNamespace

from devnet_tests.mempool_probes import NonceGapResult, saturate_pool, send_nonce_gap

ADDRESS = "0x" + "1" * 40


class _FakeAccount:
    def __init__(self, address=ADDRESS):
        self.address = address
        self.calls = []

    def sign_transaction(self, tx):
        self.calls.append(dict(tx))
        return SimpleNamespace(raw_transaction=b"raw")


def _fake_w3(send_raw_transaction=lambda raw: None, make_request=None, start_nonce=5):
    def default_make_request(method, params):
        raise AssertionError(f"unexpected RPC call: {method}")

    nonce_lookups = []

    def get_transaction_count(addr, block="latest"):
        nonce_lookups.append((addr, block))
        return start_nonce

    eth = SimpleNamespace(
        chain_id=777,
        gas_price=1000,
        get_transaction_count=get_transaction_count,
        send_raw_transaction=send_raw_transaction,
    )
    provider = SimpleNamespace(make_request=make_request or default_make_request)
    return SimpleNamespace(eth=eth, provider=provider, nonce_lookups=nonce_lookups)


def _status_response(pending=0, queued=0):
    def make_request(method, params):
        assert method == "txpool_status"
        return {"result": {"pending": hex(pending), "queued": hex(queued)}}

    return make_request


def test_send_nonce_gap_sends_start_then_start_plus_two():
    calls = []

    def send_raw_transaction(raw):
        calls.append(raw)

    account = _FakeAccount()
    w3 = _fake_w3(send_raw_transaction=send_raw_transaction, start_nonce=5)
    send_nonce_gap(w3, account)
    nonces = [tx["nonce"] for tx in account.calls]
    assert nonces == [5, 7]
    assert w3.nonce_lookups == [(ADDRESS, "pending")]


def test_send_nonce_gap_reports_rejection_of_the_gap_tx():
    def send_raw_transaction(raw):
        if len(account.calls) == 2:
            raise RuntimeError("account sequence mismatch, expected 6, got 7")

    account = _FakeAccount()
    w3 = _fake_w3(send_raw_transaction=send_raw_transaction)
    result = send_nonce_gap(w3, account)
    assert result == NonceGapResult(
        gap_tx_rejected=True,
        error="account sequence mismatch, expected 6, got 7",
    )


def test_send_nonce_gap_reports_unrejected_when_gap_tx_is_accepted():
    account = _FakeAccount()
    w3 = _fake_w3()
    result = send_nonce_gap(w3, account)
    assert result == NonceGapResult(gap_tx_rejected=False)


def test_send_nonce_gap_captures_first_send_failure_without_raising():
    def raising(raw):
        raise RuntimeError("boom")

    account = _FakeAccount()
    w3 = _fake_w3(send_raw_transaction=raising)
    result = send_nonce_gap(w3, account)
    assert result == NonceGapResult(gap_tx_rejected=False, error="boom")


def test_saturate_pool_counts_accepted_and_rejected():
    calls = []

    def send_raw_transaction(raw):
        calls.append(raw)
        if len(calls) % 2 == 0:
            raise RuntimeError("mempool is full")

    account = _FakeAccount()
    w3 = _fake_w3(
        send_raw_transaction=send_raw_transaction,
        make_request=_status_response(pending=3, queued=1),
    )
    result = saturate_pool(w3, account, batch_size=4)
    assert result.sent == 4
    assert result.accepted == 2
    assert result.rejected == 2
    assert result.pool_pending == 3
    assert result.pool_queued == 1
    assert result.sample_rejection == "mempool is full"
    assert result.error is None


def test_saturate_pool_sends_consecutive_nonces_from_current():
    account = _FakeAccount()
    w3 = _fake_w3(make_request=_status_response(), start_nonce=10)
    saturate_pool(w3, account, batch_size=3)
    nonces = [tx["nonce"] for tx in account.calls]
    assert nonces == [10, 11, 12]
    assert w3.nonce_lookups == [(ADDRESS, "pending")]


def test_saturate_pool_reports_status_query_failure_without_raising():
    account = _FakeAccount()

    def raising_status(method, params):
        raise RuntimeError("connection reset")

    w3 = _fake_w3(make_request=raising_status)
    result = saturate_pool(w3, account, batch_size=2)
    assert result.accepted == 2
    assert result.error == "connection reset"
    assert result.pool_pending == 0
    assert result.pool_queued == 0
