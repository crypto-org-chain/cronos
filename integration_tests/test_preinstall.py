from .network import Cronos
from hexbytes import HexBytes
from .cosmoscli import module_address
from .utils import CONTRACTS, deploy_contract, submit_gov_proposal

def test_preinstall(cronos: Cronos):
    """
    check preinstall functionalities
    """
    w3 = cronos.w3
    create2address = '0x4e59b44847b379578588920cA78FbF26c0B4956C'
    create2code = w3.eth.get_code(create2address)
    assert create2code == HexBytes("0x")

    create2_preinstall = {
        "name": "Create2",
        "address": create2address,
        "code": "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
    }


    msg = "/ethermint.evm.v1.MsgRegisterPreinstalls"
    authority = module_address("gov")
    submit_gov_proposal(
        cronos,
        msg,
        messages=[
            {
                "@type": msg,
                "authority": authority,
                "preinstalls": [create2_preinstall],
            }
        ],
    )

    create2code = w3.eth.get_code(create2address)
    print(f"Contract address: {create2address}")
    print(f"Contract bytecode: {create2code.hex()[:70]}...")
    assert create2code == HexBytes("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3")

