from .security_probes import send_unauthorized_cro_bridge_call


def test_unauthorized_cro_bridge_call_is_rejected(devnet, funded_account):
    result = send_unauthorized_cro_bridge_call(devnet.nodes[0].w3, funded_account)
    assert result.error is None
    assert result.rejected
    # Pin the rejection to the allowlist check rather than accepting any revert:
    # the EVM execution itself has to be clean (the hook, which the trace replay
    # skips, is what failed) and the hook failure has to have cleared the
    # contract's __CronosSendCroToIbc log.
    assert result.evm_error is None
    assert result.receipt_logs == 0
