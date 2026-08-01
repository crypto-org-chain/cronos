"""Integration tests for post-Osaka/Prague EIP behavior on the eth JSON-RPC
surface.

Devnet genesis activates OsakaTime and PragueTime at 0 (ethermint's
DefaultChainConfig), so every check below is live from genesis on the
default `cronos` fixture -- no custom jsonnet config is needed. These
tests each pin down whether the guarded behavior fails at admission
(send_raw_transaction / CheckTx, before the tx ever occupies mempool or
block space) or only at execution (block inclusion, surfaced via the
mined receipt) -- that distinction is the point of the suite, not an
incidental detail.

Covers:
  - EIP-7825: per-tx gas-limit cap, rejected at admission
    (keeper.CheckMaxTxGas is wired into the shared ante handler)
  - EIP-7623: floor data-gas cost, rejected at admission
    (keeper.VerifyFee enforces FloorDataGas unconditionally under
    Prague rules, from within ante's CheckEthGasConsume)
  - EIP-1559: fee-cap balance check, rejected at admission
    (keeper.CheckSenderBalance runs inside CheckEthGasConsume)
  - EIP-4844: blob-tx handling on submission, verifying whether it is
    cleanly rejected or silently executed with blobs dropped
"""

import time

import pytest
from eth_account import Account
from web3 import Web3

from .utils import ADDRS, KEYS, derive_new_account, send_transaction, sign_transaction

# EIP-7825 (params.MaxTxGas): 1 << 24.
MAX_TX_GAS = 16_777_216

# json-rpc `txfee-cap: 2` in configs/default.jsonnet, in wei.
RPC_TXFEE_CAP = 2 * 10**18


def _gas_price_under_fee_cap(w3, gas, attempts=12):
    """A gas price for a `gas`-limit tx that stays under the node's txfee-cap
    while still beating the base fee of the block that validates it.

    SendRawTransaction runs CheckTxFee before CheckTx, so the price is bounded
    by cap/gas, which for a cap-sized gas limit sits only a little above the
    genesis base fee. Earlier tests in the session fill blocks and push the base
    fee up, so wait for it to decay instead of assuming it is still at genesis.
    The margin required is 1/8: feemarket's BaseFeeChangeDenominator caps
    EIP-1559's per-block move at 12.5%, so a price above 9/8 of the base fee
    just read is still above the base fee one block later.
    """
    # 99% of the cap, not all of it: CheckTxFee compares the fee in ether as a
    # float, so leave room for rounding right at the boundary.
    affordable = RPC_TXFEE_CAP * 99 // (100 * gas)
    for _ in range(attempts):
        base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
        if affordable * 8 >= base_fee * 9:
            return min(max(w3.eth.gas_price, base_fee * 9 // 8), affordable)
        height = w3.eth.block_number
        deadline = time.monotonic() + 20
        while w3.eth.block_number == height and time.monotonic() < deadline:
            time.sleep(0.5)
    raise AssertionError(
        f"base fee {base_fee} never fell far enough for a {gas} gas tx to fit "
        f"under the {RPC_TXFEE_CAP} wei fee cap (max affordable gas price "
        f"{affordable})"
    )


def test_eip7825_gas_cap_rejected_at_admission(cronos):
    """A tx with gas limit above the EIP-7825 cap (1<<24) is rejected at
    eth_sendRawTransaction, not merely wasted once included in a block.

    keeper.CheckMaxTxGas runs inside ante's CheckEthGasConsume.
    """
    w3: Web3 = cronos.w3
    gas = MAX_TX_GAS + 1
    gas_price = _gas_price_under_fee_cap(w3, gas)
    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "gas": gas,
        "gasPrice": gas_price,
    }
    signed = sign_transaction(w3, tx)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed.raw_transaction)
    # core.ErrGasLimitTooHigh ("transaction gas limit too high"); match the
    # exact wording so a generic RPC/nonce/fee error can't satisfy this.
    msg = str(exc_info.value).lower()
    assert "gas limit too high" in msg, msg


def test_eip7623_floor_data_gas_rejected_at_admission(cronos):
    """A tx with data-heavy calldata and a gas limit above intrinsic gas
    but below the EIP-7623 floor is rejected at send_raw_transaction,
    the same admission-time path as the EIP-7825 cap above.

    keeper.VerifyFee runs the FloorDataGas check unconditionally
    whenever Prague rules are active, inside ante's CheckEthGasConsume.

    1000 non-zero calldata bytes: intrinsic gas is 21000 + 1000*16 =
    37000, but the floor is 21000 + 1000*4*10 = 61000 (EIP-7623 charges
    4 tokens per non-zero byte at 10 gas/token). A gas limit of 40000
    clears intrinsic gas but falls short of the floor, so admission
    fails with core.ErrFloorDataGas.
    """
    w3: Web3 = cronos.w3
    tx = {
        "to": ADDRS["community"],
        "value": 1,
        "gas": 40_000,
        "gasPrice": w3.eth.gas_price,
        "data": b"\x01" * 1000,
    }
    signed = sign_transaction(w3, tx)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed.raw_transaction)
    # core.ErrFloorDataGas ("insufficient gas for floor data gas cost"); a bare
    # "gas" or "insufficient" substring would also match the intrinsic-gas and
    # balance errors, which are different paths.
    msg = str(exc_info.value).lower()
    assert "floor data gas" in msg, msg


def test_eip1559_fee_cap_balance_check_rejected_at_admission(cronos):
    """A sender funded only for gas at the current base fee, but whose
    signed maxFeePerGas is set far above it, is rejected at admission --
    the EIP-1559 upfront cost check (gasLimit * feeCap + value) is what
    ante.CheckSenderBalance enforces, not the effective gas price the
    tx would actually pay.
    """
    w3: Web3 = cronos.w3
    gas_limit = 21000
    base = w3.eth.gas_price
    fee_cap = base + 10**12
    sender = derive_new_account(9825)
    fund_tx = {
        "to": sender.address,
        "value": gas_limit * base,
        "gasPrice": base,
    }
    send_transaction(w3, fund_tx, KEYS["validator"])
    assert w3.eth.get_balance(sender.address) == gas_limit * base

    tx = {
        "to": ADDRS["community"],
        "value": 0,
        "gas": gas_limit,
        "maxFeePerGas": fee_cap,
        "maxPriorityFeePerGas": fee_cap,
        "chainId": w3.eth.chain_id,
    }
    signed = sign_transaction(w3, tx, sender.key)
    with pytest.raises(Exception) as exc_info:
        w3.eth.send_raw_transaction(signed.raw_transaction)
    # keeper.CheckSenderBalance wraps ErrInsufficientFunds as
    # "sender balance < tx cost (...)"; pin that so a min-gas-price or
    # base-fee rejection can't pass as a balance check.
    msg = str(exc_info.value).lower()
    assert "sender balance < tx cost" in msg, msg


def _sign_blob_tx(w3, key, to):
    """Build and sign a type-3 (EIP-4844) transaction with one all-zero
    blob. An all-zero 131072-byte blob is trivially a valid sequence of
    BLS field elements (each one is the value 0), which avoids needing a
    real KZG-valid payload just to exercise submission handling.
    """
    acct = Account.from_key(key)
    blob = b"\x00" * 131072
    tx = {
        "type": 3,
        "chainId": w3.eth.chain_id,
        "nonce": w3.eth.get_transaction_count(acct.address),
        "to": to,
        "value": 0,
        "gas": 21000,
        "maxPriorityFeePerGas": w3.eth.gas_price,
        "maxFeePerGas": w3.eth.gas_price + 10**9,
        "maxFeePerBlobGas": 10**9,
        "accessList": [],
    }
    return acct.sign_transaction(tx, blobs=[blob])


# The unmarked marker is explicit here because conftest only auto-marks tests
# that carry no marker at all, and CI picks legs by marker - an xfail-only test
# would sit in no matrix leg and never run, exactly like the skip it replaces.
@pytest.mark.unmarked
@pytest.mark.xfail(
    strict=True,
    reason="the pinned ethermint (v0.22.1-0.20260716064133-ceb48bc17115) maps a "
    "type-3 tx onto newLegacyTx in NewTxDataFromTx and has no BlobTxType check in "
    "EthereumTx.Validate, so the blob tx is accepted with its blobs dropped. The "
    "check only exists in the gitignored vendor/ tree, so it is not in a binary "
    "built from this repo. strict=True so this flips to a hard failure once the "
    "fix lands in the pinned dependency. Tracked in "
    "docs/audit/devnet-tests-bugfixes-2026-07.md - needs a PR to "
    "crypto-org-chain/ethermint.",
)
def test_eip4844_blob_tx_rejected_at_admission(cronos):
    """A type-3 (blob) transaction should be cleanly rejected at
    eth_sendRawTransaction, not silently accepted with its blobs
    discarded.

    Static review of call_tx.go's SendRawTransaction and msg.go's
    AsMessage found no explicit BlobTxType check on the ethermint side:
    AsMessage never copies BlobHashes/BlobGasFeeCap onto the core.Message
    it builds, so geth's own blob checks in preCheck are structurally
    bypassed for ethermint-originated messages.
    """
    w3: Web3 = cronos.w3
    signed = _sign_blob_tx(w3, KEYS["community"], ADDRS["validator"])
    try:
        tx_hash = w3.eth.send_raw_transaction(signed.raw_transaction)
    except Exception as exc_info:
        # ErrTxTypeNotSupported ("transaction type not supported"), wrapped by
        # the vendor patch as "blob transactions are not supported".
        msg = str(exc_info).lower()
        assert "not supported" in msg, msg
        return
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=30)
    pytest.fail(
        "blob tx was accepted instead of rejected at admission: "
        f"tx_hash={tx_hash.hex()} status={receipt.status}"
    )
