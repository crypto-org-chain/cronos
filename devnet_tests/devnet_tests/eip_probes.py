from dataclasses import dataclass

from eth_account import Account

# EIP-7623's floor-gas pricing (40 gas/nonzero byte: 4 tokens/byte * 10 gas/token)
# is strictly above EIP-2028 intrinsic-gas pricing (16 gas/nonzero byte), so
# calldata of nonzero bytes alone is enough to separate the two thresholds.
# See go-ethereum params/protocol_params.go (TxDataNonZeroGasEIP2028,
# TxTokenPerNonZeroByte, TxCostFloorPerToken).
_BASE_TX_GAS = 21000
_INTRINSIC_GAS_PER_NONZERO_BYTE = 16
_FLOOR_GAS_PER_NONZERO_BYTE = 40
_FLOOR_PROBE_DATA_LEN = 300

# EIP-7825 caps a single tx's gas limit at 1<<24 once Osaka is active.
_MAX_TX_GAS = 1 << 24


@dataclass
class ProbeResult:
    accepted: bool
    error: str | None


def _submit(w3, signer, tx, **sign_kwargs) -> ProbeResult:
    """Signs and submits `tx`, folding both signing-time and RPC-time failures
    into a ProbeResult rather than letting either raise."""
    try:
        signed = signer.sign_transaction(tx, **sign_kwargs)
        w3.eth.send_raw_transaction(signed.raw_transaction)
        return ProbeResult(accepted=True, error=None)
    except Exception as exc:  # noqa: BLE001
        return ProbeResult(accepted=False, error=str(exc))


def _base_tx(w3, account, **overrides) -> dict:
    gas_price = w3.eth.gas_price
    tx = {
        "chainId": w3.eth.chain_id,
        "to": account.address,
        "value": 0,
        "gas": _BASE_TX_GAS,
        "maxFeePerGas": gas_price * 2,
        "maxPriorityFeePerGas": gas_price,
        "data": b"",
    }
    tx.update(overrides)
    # Only look the nonce up when the caller didn't supply one, so probes that
    # pick their own nonce don't pay for a pointless RPC round-trip.
    if "nonce" not in tx:
        tx["nonce"] = w3.eth.get_transaction_count(account.address, "pending")
    return tx


def send_over_max_tx_gas(w3, account) -> ProbeResult:
    """A tx over the EIP-7825 cap must be rejected with ErrGasLimitTooHigh.

    Uses the minimum fee cap that clears the base-fee check (see
    send_below_base_fee) so the huge gas limit here doesn't also require the
    funded account to hold an unusually large balance."""
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    tx = _base_tx(
        w3,
        account,
        gas=_MAX_TX_GAS + 1,
        maxFeePerGas=base_fee,
        maxPriorityFeePerGas=0,
    )
    return _submit(w3, account, tx)


def send_under_floor_data_gas(w3, account) -> ProbeResult:
    """A gas limit set just above intrinsic gas but below the EIP-7623 floor
    for the same calldata must be rejected with ErrFloorDataGas."""
    data = b"\x01" * _FLOOR_PROBE_DATA_LEN
    intrinsic = _BASE_TX_GAS + _FLOOR_PROBE_DATA_LEN * _INTRINSIC_GAS_PER_NONZERO_BYTE
    floor = _BASE_TX_GAS + _FLOOR_PROBE_DATA_LEN * _FLOOR_GAS_PER_NONZERO_BYTE
    tx = _base_tx(w3, account, gas=(intrinsic + floor) // 2, data=data)
    return _submit(w3, account, tx)


def send_below_base_fee(w3, account) -> ProbeResult:
    """A fee cap below the current base fee must be rejected by the dynamic-fee
    ante checker with 'insufficient gas prices'. No feeCap can be below a
    base fee of 0, so this probe is meaningless (and shouldn't be run) if the
    devnet's base fee is currently 0."""
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    tx = _base_tx(
        w3, account, maxFeePerGas=max(base_fee - 1, 0), maxPriorityFeePerGas=0
    )
    return _submit(w3, account, tx)


def send_insufficient_balance(w3, account) -> ProbeResult:
    """A sender with zero balance can't cover gas*feeCap+value. Depending on
    ante-decorator ordering this may be rejected either at fee deduction
    (cosmos-side) or later in go-ethereum's own buyGas/CanTransfer (EVM-side);
    both surface an "insufficient funds" style message, so callers should
    match on that substring rather than a single exact error type."""
    unfunded = Account.create()
    tx = _base_tx(w3, account, value=1, nonce=0)
    return _submit(w3, unfunded, tx)


def send_blob_tx(w3, account) -> ProbeResult:
    """cronos doesn't support EIP-4844: NewTxDataFromTx has no BlobTxType case,
    so a real blob tx should fail cleanly at decode/conversion, not get
    silently misread as a legacy tx and applied on-chain."""
    blob = b"\x00" * 131072
    tx = _base_tx(w3, account, maxFeePerBlobGas=w3.eth.gas_price)
    return _submit(w3, account, tx, blobs=[blob])
