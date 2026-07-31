from dataclasses import dataclass


@dataclass
class NonceGapResult:
    gap_tx_rejected: bool
    error: str | None = None


@dataclass
class SaturationResult:
    sent: int
    accepted: int
    rejected: int
    pool_pending: int
    pool_queued: int
    error: str | None = None
    sample_rejection: str | None = None


def _txpool_status(w3) -> tuple[int, int]:
    rsp = w3.provider.make_request("txpool_status", [])
    status = rsp["result"]
    return int(status["pending"], 16), int(status["queued"], 16)


def _sign_and_send(w3, account, nonce: int, gas_price: int) -> None:
    tx = {
        "chainId": w3.eth.chain_id,
        "to": account.address,
        "value": 0,
        "gas": 21000,
        "gasPrice": gas_price,
        "data": b"",
        "nonce": nonce,
    }
    signed = account.sign_transaction(tx)
    w3.eth.send_raw_transaction(signed.raw_transaction)


def send_nonce_gap(w3, account) -> NonceGapResult:
    """Send a tx at nonce N, then one at N+2, deliberately skipping N+1.

    cosmos-sdk's SigVerificationDecorator checks the account sequence for an
    exact match (x/auth/ante/sigverify.go), so a gapped nonce is rejected at
    submission with "account sequence mismatch" rather than being parked for
    later inclusion — this chain's txpool_content/txpool_status never expose
    a queued pool at all (ethermint's txpool API hard-codes queued as
    empty/zero), so there's no RPC-observable "queued" state to check."""
    gas_price = w3.eth.gas_price
    start = w3.eth.get_transaction_count(account.address, "pending")
    try:
        _sign_and_send(w3, account, start, gas_price)
    except Exception as exc:  # noqa: BLE001
        return NonceGapResult(gap_tx_rejected=False, error=str(exc))

    try:
        _sign_and_send(w3, account, start + 2, gas_price)
    except Exception as exc:  # noqa: BLE001
        return NonceGapResult(gap_tx_rejected=True, error=str(exc))
    return NonceGapResult(gap_tx_rejected=False)


def saturate_pool(w3, account, batch_size: int) -> SaturationResult:
    """Send `batch_size` sequential low-fee txs as fast as possible and report
    how many were accepted/rejected along with a post-burst txpool snapshot.
    The caller decides what "saturated" means for this devnet's actual pool
    size; this probe only reports counts, it doesn't assert a threshold."""
    gas_price = w3.eth.gas_price
    start = w3.eth.get_transaction_count(account.address, "pending")
    accepted = 0
    rejected = 0
    sample_rejection = None
    for i in range(batch_size):
        try:
            _sign_and_send(w3, account, start + i, gas_price)
            accepted += 1
        except Exception as exc:  # noqa: BLE001
            rejected += 1
            if sample_rejection is None:
                sample_rejection = str(exc)

    pool_pending = pool_queued = 0
    error = None
    try:
        pool_pending, pool_queued = _txpool_status(w3)
    except Exception as exc:  # noqa: BLE001
        error = str(exc)

    return SaturationResult(
        sent=batch_size,
        accepted=accepted,
        rejected=rejected,
        pool_pending=pool_pending,
        pool_queued=pool_queued,
        error=error,
        sample_rejection=sample_rejection,
    )
