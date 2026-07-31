"""Negative-path security tests for on-chain checks that lacked coverage.

  - SendCroToIbc's authorized-bridge-contract allowlist must reject calls
    from a CroBridge contract that was never approved via governance.
  - CRC21 token-mapping registration must reject the zero address and any
    address inside the reserved precompile range (< 0x0100), regardless of
    whether the denom is a source denom or an external one.
"""

import pytest

from .ibc_utils import RATIO, get_balance, ibc_transfer, prepare_network
from .utils import ADDRS, CONTRACTS, deploy_contract, send_transaction, wait_for_fn

pytestmark = pytest.mark.ibc

ZERO_ADDRESS = "0x0000000000000000000000000000000000000000"
LOW_PRECOMPILE_ADDRESS = "0x0000000000000000000000000000000000000042"


@pytest.fixture(scope="module")
def ibc(tmp_path_factory):
    """A two-chain ibc network with basetcro already funded on cronos.

    Funding is required because the CroBridge test spends basetcro out of
    signer2's account, and a fresh network starts with none there.
    """
    path = tmp_path_factory.mktemp("security-fixes-ibc")
    gen = prepare_network(path, "ibc", incentivized=False, is_ibc_transfer=True)
    network = next(gen)
    ibc_transfer(network)
    yield network
    next(gen, None)


def test_cro_bridge_contract_unauthorized_rejected(ibc):
    """An unauthorized CroBridge contract must not move funds across ibc.

    SendCroToIbcHandler checks the caller against the governance-controlled
    cro_bridge_contract_addresses allowlist before doing anything else; a
    contract that was deployed but never approved by that proposal must have
    its call reverted rather than silently bridging value.
    """
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    dst_denom = "basecro"
    src_amount = dst_amount * RATIO
    old_dst_balance = get_balance(ibc.chainmain, dst_addr, dst_denom)

    w3 = ibc.cronos.w3
    contract = deploy_contract(w3, CONTRACTS["CroBridge"])

    cli = ibc.cronos.cosmos_cli()
    params = cli.query_params()
    authorized = [a.lower() for a in params.get("cro_bridge_contract_addresses", [])]
    assert contract.address.lower() not in authorized

    tx = contract.functions.send_cro_to_crypto_org(dst_addr).build_transaction(
        {
            "from": ADDRS["signer2"],
            "value": src_amount,
        }
    )
    receipt = send_transaction(w3, tx)
    assert receipt.status == 0

    def no_balance_change():
        return get_balance(ibc.chainmain, dst_addr, dst_denom) == old_dst_balance

    wait_for_fn("no balance change", no_balance_change)
    assert get_balance(ibc.chainmain, dst_addr, dst_denom) == old_dst_balance


def test_token_mapping_rejects_zero_address(ibc):
    """RegisterOrUpdateTokenMapping must not accept the zero address.

    ensureContractCode treats any address below 0x0100 as reserved for
    precompiles and fails before ever checking for deployed code, so the
    zero address can never be mapped to a CRC21 denom.
    """
    cli = ibc.cronos.cosmos_cli()
    rsp = cli.update_token_mapping(
        "testusd", ZERO_ADDRESS, "TESTUSD", 6, from_="validator"
    )
    assert rsp["code"] != 0
    assert "precompile range" in rsp["raw_log"]


def test_token_mapping_rejects_precompile_range_address(ibc):
    """A low, in-range address must be rejected even with no deployed code.

    The precompile-range guard runs ahead of the contract-code-exists check,
    so an attacker cannot exploit an empty precompile slot by mapping a CRC21
    denom onto it before code is ever deployed there.
    """
    cli = ibc.cronos.cosmos_cli()
    rsp = cli.update_token_mapping(
        "testusd2", LOW_PRECOMPILE_ADDRESS, "TESTUSD2", 6, from_="validator"
    )
    assert rsp["code"] != 0
    assert "precompile range" in rsp["raw_log"]


def test_source_denom_token_mapping_rejects_precompile_range_address(ibc):
    """The source-denom path also runs the precompile-range guard.

    validateContractAddressForSourceDenom only checks that the contract
    matches the address embedded in the denom; ensureContractCode still runs
    afterwards, so a source denom crafted to embed a precompile-range
    address must still be rejected.
    """
    cli = ibc.cronos.cosmos_cli()
    denom = "cronos" + LOW_PRECOMPILE_ADDRESS
    rsp = cli.update_token_mapping(
        denom, LOW_PRECOMPILE_ADDRESS, "TESTUSD3", 6, from_="validator"
    )
    assert rsp["code"] != 0
    assert "precompile range" in rsp["raw_log"]
