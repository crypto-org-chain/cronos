import queue
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed

import web3
from eth_account import Account

from .utils import export_eth_account, send_transactions, sign_transaction

TEST_AMOUNT = 1000000000000000000
GAS_PRICE = 1000000000
TX_SENDING_WORKERS = 1000


def fund_test_accounts(w3, from_account, num_accounts) -> [Account]:
    accounts = []
    nonce = w3.eth.get_transaction_count(from_account.address)
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
    receipts = send_transactions(w3, txs, from_account)
    for receipt in receipts:
        assert receipt["status"] == 1
    return accounts


def start_tx_sending_workers(workers=TX_SENDING_WORKERS):
    """
    TODO: use asyncio instead of threading
    """
    q = queue.Queue()

    def worker():
        w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
        while True:
            raw = q.get()
            if raw is None:
                break
            try:
                w3.eth.send_raw_transaction(raw)
            except Exception as e:
                print("send tx failed", e)
            finally:
                q.task_done()

    for _ in range(workers):
        threading.Thread(target=worker, daemon=True).start()

    return q


def sendtx(w3: web3.Web3, acct: Account, tx_amount: int, put_tx: callable):
    initial_nonce = w3.eth.get_transaction_count(acct.address)
    print(
        "test begin, address:",
        acct.address,
        "balance:",
        w3.eth.get_balance(acct.address),
        "nonce:",
        initial_nonce,
    )

    for nonce in range(initial_nonce, initial_nonce + tx_amount):
        tx = {
            "to": "0x0000000000000000000000000000000000000000",
            "value": 1,
            "nonce": nonce,
            "gas": 21000,
            "gasPrice": GAS_PRICE,
        }
        put_tx(sign_transaction(w3, tx, acct).rawTransaction)

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
    accounts = fund_test_accounts(w3, genesis_account, num_accounts)
    q = start_tx_sending_workers()
    with ThreadPoolExecutor(max_workers=num_accounts) as executor:
        futs = (executor.submit(sendtx, w3, acct, num_txs, q.put) for acct in accounts)
        for fut in as_completed(futs):
            try:
                fut.result()
            except Exception as e:
                print("test task failed", e)
    q.join()
