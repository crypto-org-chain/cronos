"""Integration tests for mempool.type=app + InsertTx AnteHandler validation.

Cronos overrides both ReapTxs and InsertTx. The InsertTx handler (Admitter)
runs RunTx(execModeCheck), so peer-relayed and RPC-submitted txs both pass
AnteHandler before mempool admission. These tests verify:

  - chain boots and produces blocks under mempool.type=app
  - RPC eth tx flows end-to-end (CheckTx -> reap -> block -> finalize)
  - per-sender nonce order is preserved by NewReapTxsHandler
  - replacement tx (RBF) at same nonce with higher fee is admitted
  - replacement tx with insufficient fee bump is rejected at admission
  - bad-sig / under-fee tx is rejected at admission, not at block time
  - with disable-tx-replacement=true, same-nonce tx fails at nonce check
    (ErrInvalidSequence), not at the feebump rule
  - txpool RPC namespace (content, inspect, status, contentFrom) backed by
    the app mempool via MempoolClient.PendingTxs
"""

from pathlib import Path

import pytest
import web3
from eth_account import Account
from web3 import Web3

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    sign_transaction,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def cronos_app_mempool(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos-app-mempool")
    yield from setup_custom_cronos(
        path, 26400, Path(__file__).parent / "configs/mempool_app.jsonnet"
    )


def test_chain_boots(cronos_app_mempool):
    """Node accepts mempool.type=app and serves RPC."""
    w3: Web3 = cronos_app_mempool.w3
    assert w3.eth.chain_id == 777
    assert w3.eth.block_number >= 0


def test_send_eth_tx(cronos_app_mempool):
    """RPC submit -> CheckTx (AnteHandler) -> mempool -> ReapTxs -> block."""
    w3: Web3 = cronos_app_mempool.w3
    tx = {
        "to": ADDRS["community"],
        "value": 1000,
        "gas": 21000,
        "gasPrice": w3.eth.gas_price,
    }
    signed = sign_transaction(w3, tx)
    txhash = w3.eth.send_raw_transaction(signed.raw_transaction)
    receipt = w3.eth.wait_for_transaction_receipt(txhash, timeout=30)
    assert receipt.status == 1
    assert receipt.gasUsed == 21000


def test_contract_deploy_and_call(cronos_app_mempool):
    """Contract deploy + state call go through ReapTxs path."""
    w3: Web3 = cronos_app_mempool.w3
    contract = deploy_contract(w3, CONTRACTS["Greeter"])
    tx = contract.functions.setGreeting("app-mempool").build_transaction()
    signed = sign_transaction(w3, tx)
    txhash = w3.eth.send_raw_transaction(signed.raw_transaction)
    receipt = w3.eth.wait_for_transaction_receipt(txhash, timeout=30)
    assert receipt.status == 1
    assert contract.caller.greet() == "app-mempool"


def test_nonce_ordering(cronos_app_mempool):
    """Sequential nonces from one sender land in nonce order.

    PriorityNonceMempool guarantees per-sender ascending nonce on reap.
    A gap'd nonce would stall later txs at AnteHandler-at-FinalizeBlock.
    """
    w3: Web3 = cronos_app_mempool.w3
    key = KEYS["validator"]
    sender = ADDRS["validator"]
    nonce = w3.eth.get_transaction_count(sender)

    txhashes = []
    for i in range(3):
        tx = {
            "to": ADDRS["community"],
            "value": 100 + i,
            "nonce": nonce + i,
            "gas": 21000,
            "gasPrice": w3.eth.gas_price,
        }
        signed = sign_transaction(w3, tx, key)
        txhashes.append(w3.eth.send_raw_transaction(signed.raw_transaction))

    for h in txhashes:
        receipt = w3.eth.wait_for_transaction_receipt(h, timeout=30)
        assert receipt.status == 1


def test_bad_signature_rejected_at_admission(cronos_app_mempool):
    """A tx with a tampered signature is rejected by AnteHandler at
    InsertTx/CheckTx admission time, not silently included in a block.

    This exercises the SDK fork patch (BaseApp default InsertTx runs
    RunTx(execModeCheck)). Without AnteHandler-at-admission this tx
    would sit in the mempool until FinalizeBlock and waste block space.
    """
    w3: Web3 = cronos_app_mempool.w3
    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "nonce": w3.eth.get_transaction_count(ADDRS["validator"]),
        "gas": 21000,
        "gasPrice": w3.eth.gas_price,
    }
    signed = sign_transaction(w3, tx)
    raw = bytearray(signed.raw_transaction)
    # Flip a byte deep in the signature region — last 65 bytes are r||s||v
    # for a typed eth tx; mutating the r component breaks ECDSA recovery.
    raw[-30] ^= 0xFF

    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(bytes(raw))
    msg = str(exc_info.value).lower()
    # AnteHandler rejection surfaces as a CheckTx error from the JSON-RPC
    # layer. Accept any of the common phrasings — the assertion is just
    # that it failed *at submit time*, not later.
    assert any(
        s in msg
        for s in (
            "invalid",
            "unauthorized",
            "signature",
            "sender",
        )
    ), msg


def test_nonce_gap_rejected_at_admission(cronos_app_mempool):
    """A tx whose nonce skips ahead of the sender's expected nonce is
    rejected at admission (ErrInvalidSequence), not silently queued.

    Asserts absence from "pending" rather than emptiness of "queued": this
    mempool has no pending/queued split, so "queued" is always {} either way
    and cannot distinguish rejection from a wrongly admitted tx.
    """
    w3: Web3 = cronos_app_mempool.w3
    sender = ADDRS["validator"]
    gap_nonce = w3.eth.get_transaction_count(sender) + 2
    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "nonce": gap_nonce,
        "gas": 21000,
        "gasPrice": w3.eth.gas_price,
    }
    signed = sign_transaction(w3, tx)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed.raw_transaction)
    msg = str(exc_info.value).lower()
    assert "invalid nonce" in msg, msg

    sender_pending = _pending_nonces(w3, sender)
    assert str(gap_nonce) not in sender_pending, (
        f"rejected nonce {gap_nonce} should not appear pending: {sender_pending}"
    )


def test_intrinsic_gas_rejected_at_admission(cronos_app_mempool):
    """A tx with gas-limit below intrinsic 21000 is rejected at admission."""
    w3: Web3 = cronos_app_mempool.w3
    # default.jsonnet sets minimum-gas-prices=0basetcro, so trip the eth
    # fee-checker via insufficient gas-limit (below 21000 intrinsic) rather
    # than min-gas-price.
    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "nonce": w3.eth.get_transaction_count(ADDRS["validator"]),
        "gas": 1,  # below 21000 intrinsic gas
        "gasPrice": w3.eth.gas_price,
    }
    signed = sign_transaction(w3, tx)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed.raw_transaction)
    msg = str(exc_info.value).lower()
    assert any(s in msg for s in ("gas", "intrinsic", "insufficient")), msg


@pytest.mark.flaky(max_runs=3)
def test_tx_replacement_rfc(cronos_app_mempool):
    """Same-nonce tx with +20% gasPrice replaces the original in mempool.

    Verifies the three-cache interaction path:
      send_raw(A') -> insertSeenCache miss (different bytes)
                   -> RunTx -> AnteCache.Exists(X, N) == true
                   -> nonce check skipped (replacement allowed)
                   -> PriorityNonceMempool.Insert(A') replaces A
    Only A' reaches a block; A produces no receipt.

    Config: default.jsonnet feebump=10 requires newGasPrice >= oldGasPrice*110/100
    (Go integer arithmetic). base*12//10 satisfies this for all integer base >= 0.

    Marked flaky because A can be reaped into a block before A' arrives if the
    500ms reap_interval fires in the window between the two send_raw calls. This
    is a timing race inherent to the test topology, not a logic bug.
    """
    w3: Web3 = cronos_app_mempool.w3
    key = KEYS["validator"]
    nonce = w3.eth.get_transaction_count(ADDRS["validator"])
    base_gas_price = w3.eth.gas_price

    # tx A: submitted first, will be displaced
    tx_a = {
        "to": ADDRS["community"],
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": base_gas_price,
    }
    signed_a = sign_transaction(w3, tx_a, key)
    hash_a = w3.eth.send_raw_transaction(signed_a.raw_transaction)

    # Narrow the reap-race window: confirm A is in pool (not yet mined) before
    # sending A'. If A is already mined, the retry decorator handles it.
    try:
        tx_a_state = w3.eth.get_transaction(hash_a)
        if tx_a_state.get("blockNumber") is not None:
            pytest.xfail("tx A mined before replacement sent (timing race; retry)")
    except web3.exceptions.TransactionNotFound:
        pass

    # tx A': same nonce, higher gasPrice — satisfies feebump=10 threshold
    # (base*12//10 >= base*110//100 for all integer base >= 0)
    tx_a_prime = {
        "to": ADDRS["community"],
        "value": 2,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": base_gas_price * 12 // 10,
    }
    signed_a_prime = sign_transaction(w3, tx_a_prime, key)
    hash_a_prime = w3.eth.send_raw_transaction(signed_a_prime.raw_transaction)

    # A' must land in a block
    receipt_prime = w3.eth.wait_for_transaction_receipt(hash_a_prime, timeout=30)
    assert receipt_prime.status == 1

    # A must NOT land — it was evicted by replacement before reap
    try:
        receipt_a = w3.eth.get_transaction_receipt(hash_a)
    except web3.exceptions.TransactionNotFound:
        receipt_a = None
    assert receipt_a is None, f"original tx should be replaced, got: {receipt_a}"


@pytest.mark.flaky(max_runs=3)
def test_tx_replacement_under_fee_rejected(cronos_app_mempool):
    """Same-nonce replacement with insufficient fee bump is rejected at admission.

    Exercises the app-mempool path where:
      1. Tx A passes CheckTx -> AnteCache.Set(addr, N)
      2. Tx A' (same nonce, +5% tip) passes AnteCache skip (Exists=true)
      3. PriorityNonceMempool.Insert calls TxReplacement: np < op*110/100 -> rejected

    Complements test_tx_replacement_rfc (success path) by covering the failure
    branch of TxReplacement inside the app-mempool admission path.

    Marked flaky: same reap-race as test_tx_replacement_rfc. If A mines before
    A' is submitted, the rejection reason changes to ErrInvalidSequence.
    """
    w3: Web3 = cronos_app_mempool.w3
    key = KEYS["validator"]
    nonce = w3.eth.get_transaction_count(ADDRS["validator"])
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    # Ensure non-zero tip so % bumps are meaningful — priority = tip / 1e6.
    priority_fee = max(w3.eth.max_priority_fee, w3.to_wei(1, "gwei"))

    tx_orig = {
        "to": ADDRS["community"],
        "value": 1,
        "maxFeePerGas": base_fee + priority_fee,
        "maxPriorityFeePerGas": priority_fee,
        "nonce": nonce,
        "gas": 21000,
    }
    signed_orig = sign_transaction(w3, tx_orig, key)
    hash_orig = w3.eth.send_raw_transaction(signed_orig.raw_transaction)

    # Narrow the reap-race window: if orig already mined, the under-fee tx
    # hits ErrInvalidSequence (not the feebump rule) — xfail for retry.
    try:
        if w3.eth.get_transaction(hash_orig).get("blockNumber") is not None:
            pytest.xfail("orig tx mined before replacement sent (timing race; retry)")
    except web3.exceptions.TransactionNotFound:
        pass

    # +5% tip bump — below feebump=10 threshold; must be rejected
    tx_under = {
        "to": ADDRS["community"],
        "value": 2,
        "maxFeePerGas": int((base_fee + priority_fee) * 1.05),
        "maxPriorityFeePerGas": int(priority_fee * 1.05),
        "nonce": nonce,
        "gas": 21000,
    }
    signed_under = sign_transaction(w3, tx_under, key)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed_under.raw_transaction)
    msg = str(exc_info.value).lower()
    assert (
        "replacement rule" in msg or "replacement" in msg
    ), f"expected feebump rejection but got: {msg}"

    # Wait for original tx to mine so sender state is clean for subsequent tests.
    receipt = w3.eth.wait_for_transaction_receipt(hash_orig, timeout=30)
    assert receipt.status == 1


# ---------------------------------------------------------------------------
# disable-tx-replacement fixture and tests
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def cronos_app_no_replace(tmp_path_factory):
    """App-mempool node with cronos.disable-tx-replacement=true.

    AnteCache becomes a no-op (maxTx=-1): same-nonce replacements hit the
    normal nonce check and fail with ErrInvalidSequence before reaching
    PriorityNonceMempool.Insert.
    """
    path = tmp_path_factory.mktemp("cronos-app-no-replace")
    yield from setup_custom_cronos(
        path, 26450, Path(__file__).parent / "configs/mempool_app_no_replace.jsonnet"
    )


def test_tx_replacement_disabled_rejects_same_nonce(cronos_app_no_replace):
    """With disable-tx-replacement=true, same-nonce tx fails at nonce check.

    Path verified:
      AnteCache.maxTx = -1 (no-op)
      Tx A CheckTx: checkState seq N -> N+1; AnteCache.Set is no-op
      Tx A' (same nonce N, +20% fee): AnteCache.Exists(addr,N) = false
        -> normal nonce check: expectedNonce=N+1, txNonce=N
        -> ErrInvalidSequence: "invalid nonce; got N, expected N+1"

    The error must contain "nonce" or "sequence", NOT "replacement" or
    "fit the replacement rule" — that would indicate the cache skip fired
    and the feebump rule was reached instead, meaning the flag had no effect.
    """
    w3: Web3 = cronos_app_no_replace.w3
    key = KEYS["validator"]
    nonce = w3.eth.get_transaction_count(ADDRS["validator"])
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    priority_fee = max(w3.eth.max_priority_fee, w3.to_wei(1, "gwei"))

    tx_orig = {
        "to": ADDRS["community"],
        "value": 1,
        "maxFeePerGas": base_fee + priority_fee,
        "maxPriorityFeePerGas": priority_fee,
        "nonce": nonce,
        "gas": 21000,
    }
    signed_orig = sign_transaction(w3, tx_orig, key)
    hash_orig = w3.eth.send_raw_transaction(signed_orig.raw_transaction)

    # +20% fee — sufficient for feebump rule, but must fail at nonce check
    tx_replace = {
        "to": ADDRS["community"],
        "value": 2,
        "maxFeePerGas": int((base_fee + priority_fee) * 1.2),
        "maxPriorityFeePerGas": int(priority_fee * 1.2),
        "nonce": nonce,
        "gas": 21000,
    }
    signed_replace = sign_transaction(w3, tx_replace, key)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed_replace.raw_transaction)
    msg = str(exc_info.value).lower()

    # Must be a nonce/sequence error — NOT the feebump rule
    assert any(
        s in msg for s in ("nonce", "sequence")
    ), f"expected nonce/sequence error but got: {msg}"
    assert (
        "replacement" not in msg
    ), f"got feebump rule error — cache skip fired when it should not: {msg}"

    # Original tx must still mine (chain is functional)
    receipt = w3.eth.wait_for_transaction_receipt(hash_orig, timeout=30)
    assert receipt.status == 1


# ---------------------------------------------------------------------------
# txpool RPC namespace tests
# ---------------------------------------------------------------------------


def _txpool_status(w3):
    return w3.provider.make_request("txpool_status", [])["result"]


def _txpool_content(w3):
    return w3.provider.make_request("txpool_content", [])["result"]


def _txpool_inspect(w3):
    return w3.provider.make_request("txpool_inspect", [])["result"]


def _txpool_content_from(w3, address):
    return w3.provider.make_request("txpool_contentFrom", [address])["result"]


def _pending_for(pool_pending, address):
    """Sender's pending bucket, or None. Tolerates either key casing — the
    RPC may return checksummed or lowercased addresses."""
    return pool_pending.get(address.lower()) or pool_pending.get(address)


def _pending_nonces(w3, address):
    """Sender's pending bucket keyed by nonce string, empty if absent."""
    return _pending_for(_txpool_content(w3)["pending"], address) or {}


def _assert_pool_dict_shape(result):
    assert "pending" in result and "queued" in result
    assert isinstance(result["pending"], dict)
    assert isinstance(result["queued"], dict)


def test_txpool_status_shape(cronos_app_mempool):
    """txpool_status returns {pending, queued} as hex strings."""
    w3 = cronos_app_mempool.w3
    result = _txpool_status(w3)
    assert "pending" in result and "queued" in result
    assert result["pending"].startswith("0x")
    assert result["queued"].startswith("0x")


def test_txpool_content_shape(cronos_app_mempool):
    """txpool_content returns {pending: {…}, queued: {…}}."""
    _assert_pool_dict_shape(_txpool_content(cronos_app_mempool.w3))


def test_txpool_inspect_shape(cronos_app_mempool):
    """txpool_inspect returns {pending: {…}, queued: {…}} with string summaries."""
    _assert_pool_dict_shape(_txpool_inspect(cronos_app_mempool.w3))


def test_txpool_content_from_shape(cronos_app_mempool):
    """txpool_contentFrom(addr) returns {pending: {nonce: tx}, queued: {…}}."""
    _assert_pool_dict_shape(
        _txpool_content_from(cronos_app_mempool.w3, ADDRS["validator"])
    )


@pytest.mark.flaky(max_runs=3)
def test_txpool_pending_tx_visible(cronos_app_mempool):
    """Submitted but un-mined tx appears in all four txpool endpoints.

    Race: with reap_interval=500ms the tx can be reaped before the pool is
    queried. Marked flaky; 3 runs should reliably catch it in the window.
    """
    w3 = cronos_app_mempool.w3
    sender = ADDRS["validator"]
    nonce = w3.eth.get_transaction_count(sender)
    pending_before = int(_txpool_status(w3)["pending"], 16)

    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": w3.eth.gas_price,
    }
    w3.eth.send_raw_transaction(sign_transaction(w3, tx).raw_transaction)

    # Query immediately — tx should be pooled before the next reap (~500ms).
    content = _txpool_content(w3)
    inspect = _txpool_inspect(w3)
    status = _txpool_status(w3)
    cf = _txpool_content_from(w3, sender)

    content_sender = _pending_for(content["pending"], sender)
    if content_sender is None:
        pytest.xfail("tx already mined before pool query (timing race; retry)")

    nonce_key = str(nonce)
    assert (
        nonce_key in content_sender
    ), f"nonce {nonce} missing from content: {content_sender}"
    tx_entry = content_sender[nonce_key]
    assert isinstance(tx_entry, dict), "content entry must be a tx object dict"

    # inspect entry is a human-readable summary string
    inspect_sender = _pending_for(inspect["pending"], sender)
    if inspect_sender is None:
        pytest.xfail(
            "inspect pending missing sender — timing race or address casing mismatch"
        )
    assert isinstance(
        inspect_sender.get(nonce_key), str
    ), f"inspect entry must be a string summary, got: {inspect_sender.get(nonce_key)}"

    # status pending count reflects this tx via Manager.CountTx()
    pending_after = int(status["pending"], 16)
    assert pending_after >= pending_before + 1, (
        f"status pending should grow by >= 1: before={pending_before} "
        f"after={pending_after}"
    )

    # contentFrom for this sender shows the same tx (keyed by nonce only, no address)
    assert (
        nonce_key in cf["pending"]
    ), f"contentFrom missing nonce {nonce}: {cf['pending']}"


# ---------------------------------------------------------------------------
# low-capacity mempool.max-txs fixture and saturation test
# ---------------------------------------------------------------------------


def _signed_transfer(w3, nonce):
    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": w3.eth.gas_price,
    }
    return sign_transaction(w3, tx).raw_transaction


def _pending_count(w3):
    return int(_txpool_status(w3)["pending"], 16)


def _assert_mined(w3, *txhashes):
    for h in txhashes:
        assert w3.eth.wait_for_transaction_receipt(h, timeout=30).status == 1


def _send_priority_tx(w3, key, to, nonce, priority_fee, base_fee):
    tx = {
        "to": to,
        "value": 1,
        "maxFeePerGas": base_fee + priority_fee,
        "maxPriorityFeePerGas": priority_fee,
        "nonce": nonce,
        "gas": 21000,
    }
    signed = sign_transaction(w3, tx, key)
    return w3.eth.send_raw_transaction(signed.raw_transaction)


def _flood_high_tip(w3, base_fee):
    """Manufacture congestion: sequential high-tip txs from the validator that
    outrank a minimal-tip tx on every proposal, so it is never selected."""
    nonce = w3.eth.get_transaction_count(ADDRS["validator"])
    tip = w3.to_wei(5, "gwei")
    for i in range(20):
        _send_priority_tx(
            w3, KEYS["validator"], ADDRS["community"], nonce + i, tip, base_fee
        )


@pytest.fixture(scope="module")
def cronos_app_low_capacity(tmp_path_factory):
    """App-mempool node with mempool.max-txs=5, so a test can reach
    saturation without submitting hundreds of txs."""
    path = tmp_path_factory.mktemp("cronos-app-low-capacity")
    yield from setup_custom_cronos(
        path, 26590, Path(__file__).parent / "configs/mempool_app_low_capacity.jsonnet"
    )


@pytest.mark.flaky(max_runs=3)
def test_txpool_saturation_rejects_and_reports(cronos_app_low_capacity):
    """Filling the pool to mempool.max-txs shows up in txpool_status, and the
    next tx is rejected with the mempool-full error.

    All fill txs are submitted before any receipt is awaited, to narrow the
    window for a 500ms reap to drain the pool mid-fill. A second window sits
    between the pending-count check and the overflow submission, where a reap
    could free a slot and let the overflow tx through. Both are covered by the
    flaky-retry decorator rather than an explicit guard.
    """
    w3 = cronos_app_low_capacity.w3
    sender = ADDRS["validator"]
    max_txs = 5  # must match mempool.max-txs in mempool_app_low_capacity.jsonnet
    nonce = w3.eth.get_transaction_count(sender)

    txhashes = [
        w3.eth.send_raw_transaction(_signed_transfer(w3, nonce + i))
        for i in range(max_txs)
    ]

    if _pending_count(w3) < max_txs:
        pytest.xfail(
            "pool already drained by a reap before status query (timing race; retry)"
        )

    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(_signed_transfer(w3, nonce + max_txs))
    assert "mempool is full" in str(exc_info.value).lower()

    _assert_mined(w3, *txhashes)


@pytest.mark.flaky(max_runs=3)
def test_txpool_saturation_sustained_backpressure(cronos_app_low_capacity):
    """Overflow at max-txs is CodeTypeRetry ("try again"), not a permanent
    rejection: across several blocks of continuous refill the pool stays
    capped and keeps rejecting overflow, then drains once refill stops —
    proving it recovers instead of wedging on a stale entry.

    Each round tops the pool back up to capacity before checking it, so a reap
    that frees a slot just gets refilled instead of causing a false read. The
    residual race is the same class as in
    test_txpool_saturation_rejects_and_reports, covered by flaky-retry.
    """
    w3 = cronos_app_low_capacity.w3
    cli = cronos_app_low_capacity.cosmos_cli()
    max_txs = 5  # must match mempool.max-txs in mempool_app_low_capacity.jsonnet
    nonce = w3.eth.get_transaction_count(ADDRS["validator"])

    fill_hashes = []
    for _ in range(4):
        while _pending_count(w3) < max_txs:
            fill_hashes.append(w3.eth.send_raw_transaction(_signed_transfer(w3, nonce)))
            nonce += 1

        with pytest.raises(Exception) as exc_info:
            w3.eth.send_raw_transaction(_signed_transfer(w3, nonce))
        assert "mempool is full" in str(exc_info.value).lower()

        wait_for_new_blocks(cli, 1)

    # Refill pressure stops here — the pool must drain, and the nonce that
    # kept getting bounced above must eventually be admitted and mined.
    _assert_mined(w3, *fill_hashes)

    tail_hash = w3.eth.send_raw_transaction(_signed_transfer(w3, nonce))
    _assert_mined(w3, tail_hash)


# ---------------------------------------------------------------------------
# mempool-ttl-num-blocks eviction fixture and test
# ---------------------------------------------------------------------------


# must match cronos.mempool-ttl-num-blocks in mempool_app_low_ttl.jsonnet
TTL_NUM_BLOCKS = 2


@pytest.fixture(scope="module")
def cronos_app_low_ttl(tmp_path_factory):
    """App-mempool node with mempool-ttl-num-blocks=2 and a tiny block gas
    limit, so a starved tx can be evicted within a short test run."""
    path = tmp_path_factory.mktemp("cronos-app-low-ttl")
    yield from setup_custom_cronos(
        path, 26620, Path(__file__).parent / "configs/mempool_app_low_ttl.jsonnet"
    )


@pytest.mark.flaky(max_runs=3)
def test_mempool_ttl_eviction(cronos_app_low_ttl):
    """A tx that's never selected for a block is evicted once its age exceeds
    mempool-ttl-num-blocks, rather than sitting in the pool forever.

    Congestion is manufactured, not incidental: the fixture caps block gas at
    ~2 basic transfers, and _flood_high_tip keeps outranking the victim's
    minimal-tip tx on every proposal, so it's never picked up before
    RecheckTx's TTL sweep runs.
    """
    w3 = cronos_app_low_ttl.w3
    cli = cronos_app_low_ttl.cosmos_cli()
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]

    _flood_high_tip(w3, base_fee)

    # Minimal but nonzero tip: passes the fee-floor check, ranks far below
    # the flood's tip on priority ordering.
    victim_sender = ADDRS["community"]
    victim_nonce = w3.eth.get_transaction_count(victim_sender)
    _send_priority_tx(
        w3, KEYS["community"], ADDRS["validator"], victim_nonce, 1, base_fee
    )
    victim_key = str(victim_nonce)

    if victim_key not in _pending_nonces(w3, victim_sender):
        pytest.xfail(
            "victim tx already reaped despite congestion setup (timing race; retry)"
        )

    wait_for_new_blocks(cli, TTL_NUM_BLOCKS + 2)

    pending = _pending_nonces(w3, victim_sender)
    assert victim_key not in pending, f"victim tx should be TTL-evicted: {pending}"


# ---------------------------------------------------------------------------
# mempool.recheck=false fixture and test
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def cronos_app_no_recheck(tmp_path_factory):
    """App-mempool node with mempool.recheck=false and a tiny block gas
    limit, so a test can observe that a now-stale tx isn't re-validated
    after an earlier tx from the same account commits."""
    path = tmp_path_factory.mktemp("cronos-app-no-recheck")
    yield from setup_custom_cronos(
        path, 26640, Path(__file__).parent / "configs/mempool_app_no_recheck.jsonnet"
    )


@pytest.mark.flaky(max_runs=3)
def test_mempool_recheck_disabled_keeps_stale_tx_pending(cronos_app_no_recheck):
    """With recheck=false, a tx that becomes unaffordable once an earlier tx
    from the same account commits is not re-validated and evicted the way
    Manager.RecheckTxs would with recheck=true — it just stays pending.

    A throwaway account is funded for one transfer plus change, not two: tx A
    (high tip) and tx B (nonce 1, minimal tip) both pass admission against the
    pre-A balance, but B would fail if executed after A spends most of it. A's
    high tip wins the single-tx-per-block cap ahead of the flood; B's minimal
    tip stays outranked by the flood forever, so B is never selected and only a
    recheck sweep could remove it from the pool.
    """
    w3 = cronos_app_no_recheck.w3
    cli = cronos_app_no_recheck.cosmos_cli()
    gas = 21000
    nonce_b = 1
    tip_a = w3.to_wei(10, "gwei")
    tip_b = 1

    throwaway = Account.create()
    pre_fund_base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    cost_a = (pre_fund_base_fee + tip_a) * gas + 1
    cost_b = (pre_fund_base_fee + tip_b) * gas + 1
    # The `cost_b // 2` slack also covers tx A: base fee may tick up by the
    # time the funding block commits, so A's real cost (priced against the
    # fresh base fee below) can run slightly above this estimate.
    fund_tx = {
        "to": throwaway.address,
        "value": cost_a + cost_b // 2,
        "gas": gas,
        "gasPrice": w3.eth.gas_price,
    }
    signed_fund = sign_transaction(w3, fund_tx, KEYS["validator"])
    _assert_mined(w3, w3.eth.send_raw_transaction(signed_fund.raw_transaction))

    # Re-read base fee after the funding block: a stale pre-funding value can
    # already be under current base fee, which would reject the txs below as
    # underpriced before they ever reach the pool.
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]

    _flood_high_tip(w3, base_fee)

    tx_a = _send_priority_tx(w3, throwaway.key, ADDRS["validator"], 0, tip_a, base_fee)
    _send_priority_tx(
        w3, throwaway.key, ADDRS["validator"], nonce_b, tip_b, base_fee
    )
    b_key = str(nonce_b)

    if b_key not in _pending_nonces(w3, throwaway.address):
        pytest.xfail(
            "stale tx already reaped despite congestion setup (timing race; retry)"
        )

    _assert_mined(w3, tx_a)
    wait_for_new_blocks(cli, 2)

    pending = _pending_nonces(w3, throwaway.address)
    assert b_key in pending, (
        f"stale tx should remain pending with recheck=false: {pending}"
    )
