import asyncio
from pathlib import Path

import aiohttp
import eth_abi
import ujson

from .erc20 import CONTRACT_ADDRESS
from .utils import gen_account

GAS_PRICE = 1000000000
CHAIN_ID = 777
LOCAL_JSON_RPC = "http://localhost:8545"
CONNECTION_POOL_SIZE = 1024
TXS_DIR = "txs"
RECIPIENT = "0x1" + "0" * 39


def simple_transfer_tx(nonce: int):
    return {
        "to": RECIPIENT,
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": GAS_PRICE,
        "chainId": CHAIN_ID,
    }


def erc20_transfer_tx(nonce: int):
    # data is erc20 transfer function call
    data = "0xa9059cbb" + eth_abi.encode(["address", "uint256"], [RECIPIENT, 1]).hex()
    return {
        "to": CONTRACT_ADDRESS,
        "value": 0,
        "nonce": nonce,
        "gas": 51630,
        "gasPrice": GAS_PRICE,
        "chainId": CHAIN_ID,
        "data": data,
    }


TX_TYPES = {
    "simple-transfer": simple_transfer_tx,
    "erc20-transfer": erc20_transfer_tx,
}


def gen(global_seq, num_accounts, num_txs, tx_type: str) -> [str]:
    accounts = [gen_account(global_seq, i + 1) for i in range(num_accounts)]
    txs = []
    create_tx = TX_TYPES[tx_type]
    for i in range(num_txs):
        for acct in accounts:
            txs.append(acct.sign_transaction(create_tx(i)).rawTransaction.hex())
            if len(txs) % 1000 == 0:
                print("generated", len(txs), "txs for node", global_seq)

    return txs


def save(txs: [str], datadir: Path, global_seq: int):
    d = datadir / TXS_DIR
    d.mkdir(parents=True, exist_ok=True)
    path = d / f"{global_seq}.json"
    with path.open("w") as f:
        ujson.dump(txs, f)


def load(datadir: Path, global_seq: int) -> [str]:
    path = datadir / TXS_DIR / f"{global_seq}.json"
    if not path.exists():
        return

    with path.open("r") as f:
        return ujson.load(f)


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


async def send(txs):
    connector = aiohttp.TCPConnector(limit=1024)
    async with aiohttp.ClientSession(
        connector=connector, json_serialize=ujson.dumps
    ) as session:
        tasks = [asyncio.ensure_future(async_sendtx(session, raw)) for raw in txs]
        await asyncio.gather(*tasks)
