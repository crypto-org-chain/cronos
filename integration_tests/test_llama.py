from .utils import ADDRS, CONTRACTS, deploy_contract, send_transaction
from pystarport import ports
import requests
import time


def test_call(cronos):
    w3 = cronos.w3
    addr = ADDRS["validator"]
    contract = deploy_contract(w3, CONTRACTS["TestLLama"])
    prompt = ""
    seed = 2
    steps = 256
    data = {"from": addr, "gasPrice": w3.eth.gas_price, "gas": 600000}
    tx = contract.functions.inference(prompt, seed, steps).build_transaction(data)
    print("mm-tx", tx)
    receipt = send_transaction(w3, tx)
    print("mm-receipt", receipt)
    assert receipt.status == 1
    param = {
        "jsonrpc": "2.0",
        "method": "eth_call",
        "params": [
            {
                "data": tx["data"],
                "to": "0x0000000000000000000000000000000000000067",
            },
            "latest",
        ],
        "id": 1,
    }
    url = f"http://127.0.0.1:{ports.evmrpc_port(cronos.base_port(0))}"
    rsp = requests.post(url, json=[param])
    assert rsp.status_code == 200
    print(rsp.json())
