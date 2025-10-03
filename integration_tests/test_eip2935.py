from hexbytes import HexBytes

from .cosmoscli import module_address
from .network import Cronos
from .utils import CONTRACTS, deploy_contract, submit_gov_proposal, w3_wait_for_block


def test_eip2935(cronos: Cronos):
    """
    check eip2935
    """
    w3 = cronos.w3

    # Set header_hash_num to 5 in genesis, e.g we persist only the latest 5 block hashes
    # Check that there is no history if query < current_block_number - 5
    contract = deploy_contract(
        w3,
        CONTRACTS["TestEip2935"],
    )
    w3_wait_for_block(w3, w3.eth.block_number + 10, timeout=30)
    start = w3.eth.block_number
    w3_wait_for_block(w3, start + 10, timeout=30)
    for i in range(0, 4):
        stored = contract.caller.blockhashOpcode(start + i)
        assert HexBytes(stored) == HexBytes(b"\x00" * 32)

    # Deploy history storage contract
    history_storage_address = "0x0000F90827F1C53a10cb7A02335B175320002935"
    history_storage_code = w3.eth.get_code(history_storage_address)
    assert history_storage_code == HexBytes("0x")

    expected_history_storage_code = (
        "0x3373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f356001"
        "43038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611f"
        "ff60014303065500"
    )
    history_storage_preinstall = {
        "name": "HistoryStorage",
        "address": history_storage_address,
        "code": expected_history_storage_code,
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
                "preinstalls": [history_storage_preinstall],
            }
        ],
    )

    history_storage_code = w3.eth.get_code(history_storage_address)
    assert history_storage_code == HexBytes(expected_history_storage_code)

    # Check that history < current_block_number - 5 is available
    w3_wait_for_block(w3, w3.eth.block_number + 10, timeout=30)
    start = w3.eth.block_number
    w3_wait_for_block(w3, start + 10, timeout=30)
    for i in range(0, 9):
        block = w3.eth.get_block(start + i)
        stored = contract.caller.blockhashOpcode(start + i)
        assert stored == block.hash
