from .mempool_probes import saturate_pool, send_nonce_gap

SATURATION_BATCH = 300


def test_nonce_gap_rejected_at_submission(devnet, funded_account):
    result = send_nonce_gap(devnet.nodes[0].w3, funded_account)
    assert result.gap_tx_rejected
    assert "sequence" in result.error.lower()


def test_pool_saturation_reports_growth(devnet, funded_account):
    result = saturate_pool(devnet.nodes[0].w3, funded_account, SATURATION_BATCH)
    assert result.error is None
    assert result.accepted + result.rejected == result.sent
    # queued is always 0 on this chain (see send_nonce_gap's docstring), so
    # a saturated pool only ever shows up as pool_pending growing.
    assert result.pool_pending > 0
