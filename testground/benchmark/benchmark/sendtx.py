import time

import web3
from eth_account import Account

from .utils import send_transaction

TEST_AMOUNT = 1000000000000000000
GAS_PRICE = 1000000000


def fund_test_accounts(w3, from_account, num_accounts) -> [Account]:
    accounts = []
    for i in range(num_accounts):
        acct = Account.create()
        tx = {
            "to": acct.address,
            "value": TEST_AMOUNT,
            "gas": 21000,
            "gasPrice": GAS_PRICE,
        }
        receipt = send_transaction(w3, tx, from_account, wait=True)
        assert receipt.status == 1
        print("fund test account", acct.address, "balance", TEST_AMOUNT)
        accounts.append(acct)
    return accounts


def sendtx(w3: web3.Web3, acct: Account, tx_amount: int):
    print("test address", acct.address, "balance", w3.eth.get_balance(acct.address))

    initial_nonce = w3.eth.get_transaction_count(acct.address)
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
