from .security_probes import send_unauthorized_cro_bridge_call


def test_unauthorized_cro_bridge_call_is_rejected(devnet, funded_account):
    result = send_unauthorized_cro_bridge_call(devnet.nodes[0].w3, funded_account)
    assert result.error is None
    assert result.rejected
