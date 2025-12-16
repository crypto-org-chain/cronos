from hexbytes import HexBytes

from .cosmoscli import module_address
from .network import Cronos
from .utils import ADDRS, submit_gov_proposal


def test_preinstalls(cronos: Cronos):
    """
    check preinstall functionalities
    """
    w3 = cronos.w3

    txhash = w3.eth.send_transaction(
        {
            "from": ADDRS["validator"],
            "to": "0x4e59b44847b379578588920cA78FbF26c0B4956C",
            "value": 5,
        }
    )
    receipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt.status == 1
    assert receipt.gasUsed == 21000

    create2address = "0x4e59b44847b379578588920cA78FbF26c0B4956C"
    create2code = w3.eth.get_code(create2address)
    assert create2code == HexBytes("0x")

    expected_create2_code = (
        "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
        "e03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
    )
    create2_preinstall = {
        "name": "Create2",
        "address": create2address,
        "code": expected_create2_code,
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
    assert create2code == HexBytes(expected_create2_code)
