import asyncio
import base64
import itertools
import multiprocessing
import os
from collections import namedtuple
from pathlib import Path

import aiohttp
import backoff
import eth_abi
import ujson
from hexbytes import HexBytes

from . import cosmostx
from .erc20 import CONTRACT_ADDRESS
from .utils import DEFAULT_DENOM, LOCAL_RPC, gen_account, split, split_batch

GAS_PRICE = 1000000000
CHAIN_ID = 777
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


Job = namedtuple(
    "Job",
    ["chunk", "global_seq", "num_accounts", "num_txs", "tx_type", "create_tx", "batch"],
)
EthTx = namedtuple("EthTx", ["tx", "raw", "sender"])


def _do_job(job: Job):
    accounts = [gen_account(job.global_seq, i + 1) for i in range(*job.chunk)]
    acct_txs = []
    total = 0
    for acct in accounts:
        txs = []
        for i in range(job.num_txs):
            tx = job.create_tx(i)
            raw = acct.sign_transaction(tx).rawTransaction
            txs.append(EthTx(tx, raw, HexBytes(acct.address)))
            total += 1
            if total % 1000 == 0:
                print("generated", total, "txs for node", job.global_seq)

        # to keep it simple, only build batch inside the account
        txs = [
            build_cosmos_tx(*txs[start:end])
            for start, end in split_batch(len(txs), job.batch)
        ]
        acct_txs.append(txs)
    return acct_txs


def gen(global_seq, num_accounts, num_txs, tx_type: str, batch: int) -> [str]:
    chunks = split(num_accounts, os.cpu_count())
    create_tx = TX_TYPES[tx_type]
    jobs = [
        Job(chunk, global_seq, num_accounts, num_txs, tx_type, create_tx, batch)
        for chunk in chunks
    ]

    with multiprocessing.Pool() as pool:
        acct_txs = pool.map(_do_job, jobs)

    # mix the account txs together, ordered by nonce.
    all_txs = []
    for txs in itertools.zip_longest(*itertools.chain(*acct_txs)):
        all_txs += txs

    return all_txs


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


def build_cosmos_tx(*txs: EthTx) -> str:
    """
    return base64 encoded cosmos tx, support batch
    """
    msgs = [
        cosmostx.build_any(
            "/ethermint.evm.v1.MsgEthereumTx",
            cosmostx.MsgEthereumTx(
                from_=tx.sender,
                raw=tx.raw,
            ),
        )
        for tx in txs
    ]
    fee = sum(tx.tx["gas"] * tx.tx["gasPrice"] for tx in txs)
    gas = sum(tx.tx["gas"] for tx in txs)
    body = cosmostx.TxBody(
        messages=msgs,
        extension_options=[
            cosmostx.build_any("/ethermint.evm.v1.ExtensionOptionsEthereumTx")
        ],
    )
    auth_info = cosmostx.AuthInfo(
        fee=cosmostx.Fee(
            amount=[cosmostx.Coin(denom=DEFAULT_DENOM, amount=str(fee))],
            gas_limit=gas,
        )
    )
    return base64.b64encode(
        cosmostx.TxRaw(
            body=body.SerializeToString(), auth_info=auth_info.SerializeToString()
        ).SerializeToString()
    ).decode()


@backoff.on_predicate(backoff.expo, max_time=60, max_value=5)
@backoff.on_exception(backoff.expo, aiohttp.ClientError, max_time=60, max_value=5)
async def async_sendtx(session, raw):
    async with session.post(
        LOCAL_RPC,
        json={
            "jsonrpc": "2.0",
            "method": "broadcast_tx_async",
            "params": {
                "tx": raw,
            },
            "id": 1,
        },
    ) as rsp:
        data = await rsp.json()
        if "error" in data:
            print("send tx error, will retry,", data["error"])
            return False
        return True


async def send(txs):
    connector = aiohttp.TCPConnector(limit=CONNECTION_POOL_SIZE)
    async with aiohttp.ClientSession(
        connector=connector, json_serialize=ujson.dumps
    ) as session:
        tasks = [asyncio.ensure_future(async_sendtx(session, raw)) for raw in txs]
        await asyncio.gather(*tasks)
