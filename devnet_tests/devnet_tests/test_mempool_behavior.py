from .mempool_probes import saturate_pool, send_nonce_gap

SATURATION_BATCH = 300


def test_nonce_gap_rejected_at_submission(devnet, funded_account):
    result = send_nonce_gap(devnet.nodes[0].w3, funded_account)
    assert result.gap_tx_rejected
    assert "sequence" in result.error.lower()


def test_pool_absorbs_saturation_burst(devnet, funded_account):
    """A 300-tx burst from one sender is fully absorbed at submission: no
    back-pressure is observable on this devnet profile, which runs the default
    CometBFT mempool (see the note on txpool_status below)."""
    result = saturate_pool(devnet.nodes[0].w3, funded_account, SATURATION_BATCH)
    assert result.error is None
    # txpool_status's pending count reflects appmempool.MempoolClient.CountTx,
    # which is only wired up when mempool.type=app is configured; this devnet
    # profile uses the default CometBFT mempool, so pending is hard-wired to 0
    # regardless of actual pool contents. The burst being fully absorbed at
    # submission time is the observable signal of saturation on this config.
    assert result.accepted == SATURATION_BATCH, (
        f"{result.rejected}/{result.sent} txs rejected at submission: "
        f"{result.sample_rejection}"
    )
