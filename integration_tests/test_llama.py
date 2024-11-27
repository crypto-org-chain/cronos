from .utils import ADDRS, CONTRACTS, deploy_contract, send_transaction


def test_call(cronos):
    w3 = cronos.w3
    addr = ADDRS["validator"]
    contract = deploy_contract(w3, CONTRACTS["TestLLama"])
    prompt = ""
    seed = 2
    steps = 256
    data = {"from": addr, "gasPrice": w3.eth.gas_price, "gas": 600000}
    tx = contract.functions.inference(prompt, seed, steps).build_transaction(data)
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1
