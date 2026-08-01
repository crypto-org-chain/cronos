"""Negative-path security tests for on-chain checks that lacked coverage.

  - SendCroToIbc's authorized-bridge-contract allowlist must reject calls
    from a CroBridge contract that was never approved via governance, and
    must let the same call through once it is.
  - CRC21 token-mapping registration must reject the zero address and any
    address inside the reserved precompile range (< 0x0100), regardless of
    whether the denom is a source denom or an external one.
"""

import pytest

from .cosmoscli import module_address
from .ibc_utils import RATIO, ibc_transfer, prepare_network
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    send_transaction,
    submit_gov_proposal,
)

pytestmark = pytest.mark.ibc

ZERO_ADDRESS = "0x0000000000000000000000000000000000000000"
LOW_PRECOMPILE_ADDRESS = "0x0000000000000000000000000000000000000042"
# MsgUpdateTokenMapping.ValidateBasic only accepts ibc/, gravity0x and cronos0x
# denoms, so a plain label like "testusd" is rejected client-side and never
# reaches the on-chain guard these tests target.
EXTERNAL_DENOM = f"gravity{LOW_PRECOMPILE_ADDRESS}"
SOURCE_DENOM = f"cronos{LOW_PRECOMPILE_ADDRESS}"


@pytest.fixture(scope="module")
def ibc(tmp_path_factory):
    """A two-chain ibc network with basetcro already funded on cronos.

    Funding is required because the CroBridge test spends basetcro out of
    signer2's account, and a fresh network starts with none there.
    """
    path = tmp_path_factory.mktemp("security-fixes-ibc")
    gen = prepare_network(path, "ibc", incentivized=False, is_ibc_transfer=True)
    network = next(gen)
    try:
        ibc_transfer(network)
        yield network
    finally:
        next(gen, None)


def _traced_evm_error(w3, tx_hash):
    """The traced call's top-level error, or None when the EVM ran clean.

    debug_traceTransaction replays the message through ApplyMessageWithConfig
    without ethermint's PostTxProcessing hooks, so an allowlist rejection —
    which happens inside the hook — leaves no trace error, while a plain revert
    or out-of-gas does. That is what separates the two otherwise identical
    `status == 0` receipts.
    """
    rsp = w3.provider.make_request(
        "debug_traceTransaction", [w3.to_hex(tx_hash), {"tracer": "callTracer"}]
    )
    # A missing debug namespace is a problem worth failing on, not a reason to
    # fall back to the loose status-only check.
    assert "result" in rsp, f"debug_traceTransaction unavailable: {rsp}"
    return rsp["result"].get("error")


def _bridge_call_tx(contract, dst_addr, src_amount):
    return contract.functions.send_cro_to_crypto_org(dst_addr).build_transaction(
        {
            "from": ADDRS["signer2"],
            "value": src_amount,
        }
    )


def test_cro_bridge_contract_unauthorized_rejected(ibc):
    """An unauthorized CroBridge contract must not move funds across ibc.

    SendCroToIbcHandler checks the caller against the governance-controlled
    cro_bridge_contract_addresses allowlist before doing anything else; a
    contract that was deployed but never approved by that proposal must have
    its call reverted rather than silently bridging value.
    """
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    src_amount = dst_amount * RATIO

    w3 = ibc.cronos.w3
    contract = deploy_contract(w3, CONTRACTS["CroBridge"])

    cli = ibc.cronos.cosmos_cli()
    params = cli.query_params()
    # PrintProto marshals with EmitDefaults, so an empty allowlist is still
    # present as []; an absent key means the params shape changed and the
    # precondition below would pass without checking anything.
    assert "cro_bridge_contract_addresses" in params, params
    authorized = [a.lower() for a in params["cro_bridge_contract_addresses"]]
    assert contract.address.lower() not in authorized

    receipt = send_transaction(w3, _bridge_call_tx(contract, dst_addr, src_amount))
    assert receipt.status == 0
    # Pin the failure to the allowlist check rather than accepting any revert:
    # the EVM execution itself has to be clean, and the hook failure has to
    # have cleared the contract's __CronosSendCroToIbc log
    # (state_transition.go sets res.Logs = nil on a PostTxProcessing error).
    assert _traced_evm_error(w3, receipt.transactionHash) is None
    assert len(receipt.logs) == 0
    # No balance check on the destination chain: ibc delivery is relayer-async,
    # so an unchanged balance is equally consistent with "still in flight" and
    # proves nothing about the rejection.


def test_cro_bridge_contract_authorized_accepted(ibc):
    """Positive control for the rejection above: the same contract bytecode and
    the same call succeed once governance adds the address to the allowlist, so
    the rejection is attributable to the allowlist and not to a broken bridge
    setup.

    Value actually landing on the other chain is already covered by
    test_ibc.test_cro_bridge_contract; here it is enough that the tx commits
    with its __CronosSendCroToIbc log intact, which is exactly what the
    unauthorized run loses.
    """
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    src_amount = 2 * RATIO

    w3 = ibc.cronos.w3
    contract = deploy_contract(w3, CONTRACTS["CroBridge"])

    cli = ibc.cronos.cosmos_cli()
    params = cli.query_params()
    params["cro_bridge_contract_addresses"] = [contract.address]
    msg = "/cronos.MsgUpdateParams"
    submit_gov_proposal(
        ibc.cronos,
        msg,
        messages=[
            {
                "@type": msg,
                "authority": module_address("gov"),
                "params": params,
            }
        ],
    )
    stored = [
        a.lower() for a in cli.query_params().get("cro_bridge_contract_addresses", [])
    ]
    assert contract.address.lower() in stored, stored

    receipt = send_transaction(w3, _bridge_call_tx(contract, dst_addr, src_amount))
    assert receipt.status == 1
    # The hook succeeded, so the __CronosSendCroToIbc log survives instead of
    # being dropped the way the unauthorized run's is.
    assert len(receipt.logs) >= 1, receipt.logs


def test_token_mapping_rejects_zero_address(ibc):
    """RegisterOrUpdateTokenMapping must not accept the zero address.

    ensureContractCode treats any address below 0x0100 as reserved for
    precompiles and fails before ever checking for deployed code, so the
    zero address can never be mapped to a CRC21 denom.
    """
    cli = ibc.cronos.cosmos_cli()
    rsp = cli.update_token_mapping(
        EXTERNAL_DENOM, ZERO_ADDRESS, "TESTUSD", 6, from_="validator"
    )
    assert rsp["code"] != 0
    assert "precompile range" in rsp["raw_log"]


@pytest.mark.parametrize(
    "denom,symbol",
    [
        (EXTERNAL_DENOM, "TESTUSD2"),
        (SOURCE_DENOM, "TESTUSD3"),
    ],
    ids=["external-denom", "source-denom"],
)
def test_token_mapping_rejects_precompile_range_address(ibc, denom, symbol):
    """A low, in-range address must be rejected even with no deployed code,
    on both the external-denom and source-denom registration paths.

    The precompile-range guard (ensureContractCode) runs ahead of the
    contract-code-exists check, so an attacker cannot exploit an empty
    precompile slot by mapping a CRC21 denom onto it before code is ever
    deployed there. On the source-denom path it also runs ahead of
    validateContractAddressForSourceDenom (which only checks that the
    contract matches the address embedded in the denom), so a source denom
    crafted to embed a precompile-range address must still be rejected.
    """
    cli = ibc.cronos.cosmos_cli()
    rsp = cli.update_token_mapping(
        denom, LOW_PRECOMPILE_ADDRESS, symbol, 6, from_="validator"
    )
    assert rsp["code"] != 0
    assert "precompile range" in rsp["raw_log"]


def test_token_mapping_accepts_deployed_contract(ibc):
    """Positive control for the two rejection tests above: the same call with
    a legitimate contract address must be accepted, proving the rejections
    come from the precompile-range guard and not from a broken test setup.
    """
    w3 = ibc.cronos.w3
    contract = deploy_contract(w3, CONTRACTS["TestERC20Utility"])
    assert int(contract.address, 16) >= 0x100

    cli = ibc.cronos.cosmos_cli()
    rsp = cli.update_token_mapping(
        f"gravity{contract.address}", contract.address, "TESTOK", 6, from_="validator"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
