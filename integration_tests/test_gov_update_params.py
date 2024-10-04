import hashlib
import json

import pytest

from .utils import CONTRACTS, approve_proposal, deploy_contract, eth_to_bech32

pytestmark = pytest.mark.gov


def test_evm_update_param(cronos, tmp_path):
    contract = deploy_contract(
        cronos.w3,
        CONTRACTS["Random"],
    )
    res = contract.caller.randomTokenId()
    assert res > 0, res
    cli = cronos.cosmos_cli()
    p = cli.query_params("evm")
    del p["chain_config"]["merge_netsplit_block"]
    del p["chain_config"]["shanghai_time"]
    proposal = tmp_path / "proposal.json"
    # governance module account as signer
    data = hashlib.sha256("gov".encode()).digest()[:20]
    signer = eth_to_bech32(data)
    proposal_src = {
        "messages": [
            {
                "@type": "/ethermint.evm.v1.MsgUpdateParams",
                "authority": signer,
                "params": p,
            }
        ],
        "deposit": "1basetcro",
        "title": "title",
        "summary": "summary",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cli.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cronos, rsp["events"])
    print("check params have been updated now")
    p = cli.query_params("evm")
    assert not p["chain_config"]["merge_netsplit_block"]
    assert not p["chain_config"]["shanghai_time"]
    invalid_msg = "invalid opcode: PUSH0"
    with pytest.raises(ValueError) as e_info:
        contract.caller.randomTokenId()
    assert invalid_msg in str(e_info.value)
    with pytest.raises(ValueError) as e_info:
        deploy_contract(cronos.w3, CONTRACTS["Greeter"])
    assert invalid_msg in str(e_info.value)


def test_gov_update_params(cronos, tmp_path):
    cli = cronos.cosmos_cli()

    proposal = tmp_path / "proposal.json"
    # governance module account as signer
    signer = "crc10d07y265gmmuvt4z0w9aw880jnsr700jdufnyd"
    params = {
        "cronos_admin": "crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp",
        "enable_auto_deployment": False,
        "ibc_cro_denom": "ibc/6411AE2ADA1E73DB59DB151"
        "A8988F9B7D5E7E233D8414DB6817F8F1A01600000",
        "ibc_timeout": "96400000000000",
        "max_callback_gas": "400000",
    }
    proposal_src = {
        "messages": [
            {
                "@type": "/cronos.MsgUpdateParams",
                "authority": signer,
                "params": params,
            }
        ],
        "deposit": "1basetcro",
        "title": "title",
        "summary": "summary",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cli.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cronos, rsp["events"])
    print("check params have been updated now")
    rsp = cli.query_params()
    print("params", rsp)
    assert rsp == params
