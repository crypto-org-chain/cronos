"""Integration tests for mempool.type=app + InsertTx AnteHandler validation.

Cronos overrides both ReapTxs and InsertTx. The InsertTx handler (Admitter)
runs RunTx(execModeCheck), so peer-relayed and RPC-submitted txs both pass
AnteHandler before mempool admission. These tests verify:

  - chain boots and produces blocks under mempool.type=app
  - RPC eth tx flows end-to-end (CheckTx -> reap -> block -> finalize)
  - per-sender nonce order is preserved by NewReapTxsHandler
  - replacement tx (RBF) at same nonce with higher fee is admitted
  - bad-sig / under-fee tx is rejected at admission, not at block time
"""

from pathlib import Path

import pytest
import web3
from web3 import Web3

from .network import setup_custom_cronos
from .utils import ADDRS, CONTRACTS, KEYS, deploy_contract, sign_transaction

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
