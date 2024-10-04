import pytest

from .cosmoscli import module_address
from .utils import CONTRACTS, deploy_contract, submit_gov_proposal

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
    authority = module_address("gov")
    submit_gov_proposal(
        cronos,
        tmp_path,
        messages=[
            {
                "@type": "/ethermint.evm.v1.MsgUpdateParams",
                "authority": authority,
                "params": p,
            }
        ],
    )
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
    params = {
        "cronos_admin": "crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp",
        "enable_auto_deployment": False,
        "ibc_cro_denom": "ibc/6411AE2ADA1E73DB59DB151"
        "A8988F9B7D5E7E233D8414DB6817F8F1A01600000",
        "ibc_timeout": "96400000000000",
        "max_callback_gas": "400000",
    }
    authority = module_address("gov")
    submit_gov_proposal(
        cronos,
        tmp_path,
        messages=[
            {
                "@type": "/cronos.MsgUpdateParams",
                "authority": authority,
                "params": params,
            }
        ],
    )
    assert cronos.cosmos_cli().query_params() == params
