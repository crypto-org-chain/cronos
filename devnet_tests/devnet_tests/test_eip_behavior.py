import pytest

from .eip_probes import (
    send_below_base_fee,
    send_blob_tx,
    send_insufficient_balance,
    send_over_max_tx_gas,
    send_under_floor_data_gas,
)


def test_max_tx_gas_rejected(devnet, funded_account):
    result = send_over_max_tx_gas(devnet.nodes[0].w3, funded_account)
    assert not result.accepted
    assert "gas limit too high" in result.error


def test_floor_data_gas_rejected(devnet, funded_account):
    result = send_under_floor_data_gas(devnet.nodes[0].w3, funded_account)
    assert not result.accepted
    assert "floor data gas" in result.error


def test_below_base_fee_rejected(devnet, funded_account):
    w3 = devnet.nodes[0].w3
    if w3.eth.get_block("latest")["baseFeePerGas"] == 0:
        pytest.skip("base fee is currently 0, no feeCap can be below it")
    result = send_below_base_fee(w3, funded_account)
    assert not result.accepted
    assert "max fee per gas less than block base fee" in result.error


def test_insufficient_balance_rejected(devnet, funded_account):
    result = send_insufficient_balance(devnet.nodes[0].w3, funded_account)
    assert not result.accepted
    assert "insufficient" in result.error.lower()


@pytest.mark.skip(
    reason="needs the EthereumTx.Validate() type check in the ethermint fork; the "
    "fix currently only exists in the gitignored vendor/ tree, so it isn't in a "
    "binary built from this repo. Tracked in "
    "docs/audit/devnet-tests-bugfixes-2026-07.md — needs a PR to "
    "crypto-org-chain/ethermint."
)
def test_blob_tx_rejected(devnet, funded_account):
    result = send_blob_tx(devnet.nodes[0].w3, funded_account)
    assert not result.accepted, (
        "blob tx was accepted instead of rejected - cronos may be silently "
        "misinterpreting EIP-4844 fields as a legacy transaction"
    )
