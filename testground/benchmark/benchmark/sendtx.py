import asyncio
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import aiohttp
import ujson
import web3
from eth_account import Account

from .utils import gen_account, send_transaction

GAS_PRICE = 1000000000
CHAIN_ID = 777
LOCAL_JSON_RPC = "http://localhost:8545"
CONNECTION_POOL_SIZE = 1024


def test_tx(nonce: int):
    return {
        "to": "0x0000000000000000000000000000000000000000",
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": GAS_PRICE,
        "chainId": CHAIN_ID,
    }


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
        try:
            send_transaction(w3, test_tx(nonce), acct, wait=False)
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


def generate_load(num_accounts, num_txs, global_seq, **kwargs):
    w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
    assert w3.eth.chain_id == CHAIN_ID
    accounts = [gen_account(global_seq, i + 1) for i in range(num_accounts)]
    with ThreadPoolExecutor(max_workers=num_accounts) as executor:
        futs = (executor.submit(sendtx, w3, acct, num_txs) for acct in accounts)
        for fut in as_completed(futs):
            try:
                fut.result()
            except Exception as e:
                print("test task failed", e)


def prepare_txs(global_seq, num_accounts, num_txs):
    accounts = [gen_account(global_seq, i + 1) for i in range(num_accounts)]
    txs = []
    for i in range(num_txs):
        for acct in accounts:
            txs.append(acct.sign_transaction(test_tx(i)).rawTransaction.hex())
            if len(txs) % 1000 == 0:
                print("prepared", len(txs), "txs")

    return txs


async def async_sendtx(session, raw):
    async with session.post(
        LOCAL_JSON_RPC,
        json={
            "jsonrpc": "2.0",
            "method": "eth_sendRawTransaction",
            "params": [raw],
            "id": 1,
        },
    ) as rsp:
        data = await rsp.json()
        if "error" in data:
            print("send tx error", data["error"])


async def send_txs(txs):
    connector = aiohttp.TCPConnector(limit=1024)
    async with aiohttp.ClientSession(
        connector=connector, json_serialize=ujson.dumps
    ) as session:
        tasks = [asyncio.ensure_future(async_sendtx(session, raw)) for raw in txs]
        await asyncio.gather(*tasks)
