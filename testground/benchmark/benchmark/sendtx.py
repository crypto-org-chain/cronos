import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import web3
from eth_account import Account

from .utils import gen_account, send_transaction

GAS_PRICE = 1000000000


def sendtx(w3: web3.Web3, acct: Account, tx_amount: int):
    initial_nonce = w3.eth.get_transaction_count(acct.address)
    print(
        "test begin, address:",
        acct.address,
        "balance:",
        w3.eth.get_balance(acct.address),
        "nonce:",
        initial_nonce,
    )

    nonce = initial_nonce
    while nonce < initial_nonce + tx_amount:
        tx = {
            "to": "0x0000000000000000000000000000000000000000",
            "value": 1,
            "nonce": nonce,
            "gas": 21000,
            "gasPrice": GAS_PRICE,
        }
        try:
            send_transaction(w3, tx, acct, wait=False)
        except ValueError as e:
            msg = str(e)
            if "invalid nonce" in msg:
                print("invalid nonce and retry", nonce)
                time.sleep(1)
                continue
            if "tx already in mempool" not in msg:
                raise

        nonce += 1

        if nonce % 100 == 0:
            print(f"{acct.address} sent {nonce} transactions")

    print(
        "test end, address:",
        acct.address,
        "balance:",
        w3.eth.get_balance(acct.address),
        "nonce:",
        w3.eth.get_transaction_count(acct.address),
    )


def generate_load(cli, num_accounts, num_txs, global_seq, **kwargs):
    w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
    assert w3.eth.chain_id == 777
    accounts = [gen_account(global_seq, i + 1) for i in range(num_accounts)]
    with ThreadPoolExecutor(max_workers=num_accounts) as executor:
        futs = (executor.submit(sendtx, w3, acct, num_txs) for acct in accounts)
        for fut in as_completed(futs):
            try:
                fut.result()
            except Exception as e:
                print("test task failed", e)
