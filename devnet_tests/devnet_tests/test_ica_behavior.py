from .ica_probes import register_ica_with_unknown_connection


def test_register_ica_with_unknown_connection_is_rejected(devnet, funded_account):
    result = register_ica_with_unknown_connection(devnet.nodes[0].w3, funded_account)
    assert result.error is None
    assert result.rejected
