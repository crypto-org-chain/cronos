import pytest

from .ica_probes import register_ica_with_unknown_connection

# The ica precompile (0x66) was removed from app.go's CustomContractFn list in
# 6d9579f2 ("chore: remove unused precompiles"), which also skipped the
# equivalent integration_tests/test_ica_precompile.py. Calling an address with
# no registered precompile and no contract code is just a plain CALL to an
# empty account, which the EVM reports as a no-op success (receipt.status==1,
# no logs) rather than a revert - there's no handler left to reject anything.
pytest.skip("ica precompile is not registered in app.go, see 6d9579f2", allow_module_level=True)


def test_register_ica_with_unknown_connection_is_rejected(devnet, funded_account):
    result = register_ica_with_unknown_connection(devnet.nodes[0].w3, funded_account)
    assert result.error is None
    assert result.rejected
