import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import web3
from eth_account import Account

from .utils import (
    broadcast_tx_json,
    build_batch_tx,
    export_eth_account,
    send_transaction,
)

TEST_AMOUNT = 1000000000000000000
GAS_PRICE = 1000000000


def fund_test_accounts(cli, w3, from_account, num_accounts, **kwargs) -> [Account]:
    accounts = []
    sender = from_account.address
    nonce = w3.eth.get_transaction_count(sender)
    txs = []
    for i in range(num_accounts):
        acct = Account.create()
        tx = {
            "to": acct.address,
            "value": TEST_AMOUNT,
            "gas": 21000,
            "gasPrice": GAS_PRICE,
            "nonce": nonce + i,
        }
        txs.append(tx)
        accounts.append(acct)
    cosmos_tx, _ = build_batch_tx(w3, cli, txs, from_account, **kwargs)
    rsp = broadcast_tx_json(cli, cosmos_tx, **kwargs)
    assert rsp["code"] == 0, rsp["raw_log"]
    return accounts


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


def generate_load(cli, num_accounts, num_txs, **kwargs):
    w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
    assert w3.eth.chain_id == 777
    genesis_account = export_eth_account(cli, "account", **kwargs)
    accounts = fund_test_accounts(cli, w3, genesis_account, num_accounts, **kwargs)
    with ThreadPoolExecutor(max_workers=num_accounts) as executor:
        futs = (executor.submit(sendtx, w3, acct, num_txs) for acct in accounts)
        for fut in as_completed(futs):
            try:
                fut.result()
            except Exception as e:
                print("test task failed", e)
