import asyncio
import base64
import itertools
import multiprocessing
import os
import sys
from collections import namedtuple
from pathlib import Path

import aiohttp
import backoff
import eth_abi
import ujson
from eth_account._utils.legacy_transactions import Transaction
from hexbytes import HexBytes

from . import cosmostx
from .erc20 import CONTRACT_ADDRESS
from .utils import DEFAULT_DENOM, gen_account, split, split_batch

GAS_PRICE = 1000000000
CHAIN_ID = 777
CONNECTION_POOL_SIZE = 1024
TXS_DIR = "txs"

Job = namedtuple(
    "Job",
    [
        "chunk",
        "global_seq",
        "num_txs",
        "tx_type",
        "create_tx",
        "batch",
        "nonce",
        "msg_version",
        "tx_options",
        "evm_denom",
        "wire_format",
        "sender_strategy",
        "start_account",
    ],
)
EthTx = namedtuple("EthTx", ["tx", "raw", "sender"])


def simple_transfer_tx(sender: str, nonce: int, options: dict):
    return {
        "to": sender,
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": options.get("gas_price", GAS_PRICE),
        "chainId": options.get("chain_id", CHAIN_ID),
    }


def erc20_transfer_tx(sender: str, nonce: int, options: dict):
    # data is erc20 transfer function call
    data = "0xa9059cbb" + eth_abi.encode(["address", "uint256"], [sender, 1]).hex()
    return {
        "to": CONTRACT_ADDRESS,
        "value": 0,
        "nonce": nonce,
        "gas": 51630,
        "gasPrice": options.get("gas_price", GAS_PRICE),
        "chainId": options.get("chain_id", CHAIN_ID),
        "data": data,
    }


TX_TYPES = {
    "simple-transfer": simple_transfer_tx,
    "erc20-transfer": erc20_transfer_tx,
}


def build_evm_msg_1_3(tx: EthTx):
    """
    build cronos v1.3 version of MsgEthereumTx
    """
    txn = Transaction.from_bytes(tx.raw)
    return cosmostx.build_any(
        cosmostx.MsgEthereumTx.MSG_URL,
        cosmostx.MsgEthereumTx(
            data=cosmostx.build_any(
                cosmostx.LegacyTx.MSG_URL,
                cosmostx.LegacyTx(
                    nonce=txn.nonce,
                    gas_price=str(txn.gasPrice),
                    gas=txn.gas,
                    to=txn.to.hex(),
                    value=str(txn.value),
                    data=txn.data,
                    v=txn.v.to_bytes(32, byteorder="big"),
                    r=txn.r.to_bytes(32, byteorder="big"),
                    s=txn.s.to_bytes(32, byteorder="big"),
                ),
            ),
            deprecated_hash=txn.hash().hex(),
            from_=tx.sender,
        ),
    )


def build_evm_msg_1_4(tx: EthTx):
    return cosmostx.build_any(
        cosmostx.MsgEthereumTx.MSG_URL,
        cosmostx.MsgEthereumTx(
            from_=tx.sender,
            raw=tx.raw,
        ),
    )


MSG_VERSIONS = {
    "1.3": build_evm_msg_1_3,
    "1.4": build_evm_msg_1_4,
}


def _do_job(job: Job):
    acct_txs = []
    total = 0
    for account_index in range(*job.chunk):
        txs = []
        acct = None
        if job.sender_strategy == "reuse":
            acct = gen_account(job.global_seq, account_index)

        for i in range(job.num_txs):
            if job.sender_strategy == "unique-per-tx":
                sender_index = (
                    job.start_account
                    + (account_index - job.start_account) * job.num_txs
                    + i
                )
                tx_nonce = job.nonce
                acct = gen_account(job.global_seq, sender_index)
            else:
                tx_nonce = job.nonce + i

            tx = job.create_tx(acct.address, tx_nonce, job.tx_options)
            raw = acct.sign_transaction(tx).rawTransaction
            txs.append(EthTx(tx, raw, HexBytes(acct.address)))
            total += 1

        if job.wire_format == "eth":
            txs = [HexBytes(tx.raw).hex() for tx in txs]
        else:
            # to keep it simple, only build batch inside the account
            txs = [
                build_cosmos_tx(
                    *txs[start:end],
                    msg_version=job.msg_version,
                    evm_denom=job.evm_denom,
                )
                for start, end in split_batch(len(txs), job.batch)
            ]
        acct_txs.append(txs)
    print(
        f"generated {total} EVM txs for accounts {job.chunk[0]}-{job.chunk[1] - 1}",
        file=sys.stderr,
    )
    return acct_txs


def gen(
    global_seq,
    num_accounts,
    num_txs,
    tx_type: str,
    batch: int,
    nonce: int = 0,
    start_account: int = 0,
    msg_version: str = "1.4",
    tx_options: dict = None,
    evm_denom: str = DEFAULT_DENOM,
    wire_format: str = "cosmos",
    sender_strategy: str = "reuse",
) -> [str]:
    tx_options = tx_options or {}
    chunks = split(num_accounts, os.cpu_count())
    create_tx = TX_TYPES[tx_type]
    jobs = [
        Job(
            (start + start_account, end + start_account),
            global_seq,
            num_txs,
            tx_type,
            create_tx,
            batch,
            nonce,
            msg_version,
            tx_options,
            evm_denom,
            wire_format,
            sender_strategy,
            start_account,
        )
        for start, end in chunks
    ]

    with multiprocessing.Pool() as pool:
        acct_txs = pool.map(_do_job, jobs)

    # mix the account txs together, ordered by nonce.
    all_txs = []
    for txs in itertools.zip_longest(*itertools.chain(*acct_txs)):
        all_txs += txs

    return all_txs


def physical_account_range(start: int, end: int, num_txs: int, sender_strategy: str):
    """Return the inclusive sender range needed for a logical account range."""
    if sender_strategy == "unique-per-tx":
        return start, start + (end - start + 1) * num_txs - 1
    return start, end


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


def build_cosmos_tx(*txs: EthTx, msg_version="1.4", evm_denom=DEFAULT_DENOM) -> str:
    """
    return base64 encoded cosmos tx, support batch
    """
    build_msg = MSG_VERSIONS[msg_version]
    msgs = [build_msg(tx) for tx in txs]
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
            amount=[cosmostx.Coin(denom=evm_denom, amount=str(fee))],
            gas_limit=gas,
        )
    )
    return base64.b64encode(
        cosmostx.TxRaw(
            body=body.SerializeToString(), auth_info=auth_info.SerializeToString()
        ).SerializeToString()
    ).decode()


def json_rpc_send_body(raw, method="broadcast_tx_async"):
    return {
        "jsonrpc": "2.0",
        "method": method,
        "params": {"tx": raw},
        "id": 1,
    }


def eth_send_raw_body(raw):
    return {
        "jsonrpc": "2.0",
        "method": "eth_sendRawTransaction",
        "params": [raw],
        "id": 1,
    }


@backoff.on_predicate(backoff.expo, max_time=60, max_value=5)
@backoff.on_exception(backoff.expo, aiohttp.ClientError, max_time=60, max_value=5)
async def async_sendtx(session, raw, rpc, sync=False, mode="cosmos"):
    if mode == "eth":
        async with session.post(rpc, json=eth_send_raw_body(raw)) as rsp:
            data = await rsp.json()
            if "error" in data:
                print("send tx error, will retry,", data["error"])
                return False
            return True

    method = "broadcast_tx_sync" if sync else "broadcast_tx_async"
    async with session.post(rpc, json=json_rpc_send_body(raw, method)) as rsp:
        data = await rsp.json()
        if "error" in data:
            # "tx already exists in cache" means this exact tx hash was already
            # accepted by a prior attempt (still pending or already committed -
            # CometBFT's mempool cache doesn't distinguish the two). Retrying it
            # can never succeed differently, so treat it as success rather than
            # burning up to 60s of backoff per tx chasing an already-done send.
            if "already exists in cache" in str(data["error"].get("data", "")):
                return True
            print("send tx error, will retry,", data["error"])
            return False
        result = data["result"]
        if result["code"] != 0:
            print("tx is invalid, won't retry,", result["log"])
        return True


async def send(
    txs,
    rpc,
    sync=False,
    batch_size=500,
    batch_interval=0.5,
    mode="cosmos",
    probe_batches=1,
):
    """Send transactions to a single rpc endpoint in rate-limited batches.

    Sends ``batch_size`` txs concurrently, pauses ``batch_interval`` seconds,
    then sends the next batch.  The pause lets the chain produce blocks and
    drain the mempool between batches, preventing the CheckTx flood that
    overwhelms the proposer and causes repeated consensus round timeouts.

    The first ``probe_batches`` batches are always sent with
    ``broadcast_tx_sync``/``eth_sendRawTransaction`` regardless of ``sync``,
    so CheckTx rejections are surfaced immediately. Plain
    ``broadcast_tx_async`` returns before CheckTx even runs, so a whole run
    can silently reject every tx (e.g. a bad nonce or unsupported batch
    encoding) while every send still "succeeds" and reports look like
    zero-load runs with no error printed anywhere.
    """
    connector = aiohttp.TCPConnector(limit=CONNECTION_POOL_SIZE)
    async with aiohttp.ClientSession(
        connector=connector, json_serialize=ujson.dumps
    ) as session:
        for i in range(0, len(txs), batch_size):
            chunk = txs[i : i + batch_size]
            batch_sync = sync or (i // batch_size) < probe_batches
            tasks = [
                asyncio.ensure_future(async_sendtx(session, raw, rpc, batch_sync, mode))
                for raw in chunk
            ]
            await asyncio.gather(*tasks)
            if i + batch_size < len(txs) and batch_interval > 0:
                await asyncio.sleep(batch_interval)


async def send_round_robin(
    txs,
    rpcs: [str],
    sync=False,
    batch_size=500,
    batch_interval=0.5,
    mode="cosmos",
    num_accounts=1,
    probe_batches=1,
):
    """Send transactions across multiple rpc endpoints in rate-limited batches.

    ``txs`` is laid out by ``gen()`` as consecutive nonce-rounds interleaved
    across accounts, i.e. position ``p`` belongs to account ``p %
    num_accounts``. Txs are assigned to endpoints round-robin by that account
    index (``rpcs[(i % num_accounts) % len(rpcs)]``) so every node always
    sees a given account's txs in nonce order - splitting one account's
    sequential nonces across nodes round-robin by flat position would let a
    later nonce reach a node before an earlier one propagates, which the
    node's CheckTx rejects as a nonce gap. Falls back to plain ``send`` when
    only one endpoint is configured.

    See ``send``'s docstring for why ``probe_batches`` exists.
    """
    if len(rpcs) == 1:
        return await send(
            txs,
            rpcs[0],
            sync=sync,
            batch_size=batch_size,
            batch_interval=batch_interval,
            mode=mode,
            probe_batches=probe_batches,
        )

    connector = aiohttp.TCPConnector(limit=CONNECTION_POOL_SIZE)
    async with aiohttp.ClientSession(
        connector=connector, json_serialize=ujson.dumps
    ) as session:
        for i in range(0, len(txs), batch_size):
            chunk = txs[i : i + batch_size]
            batch_sync = sync or (i // batch_size) < probe_batches
            tasks = [
                asyncio.ensure_future(
                    async_sendtx(
                        session,
                        raw,
                        rpcs[((i + j) % num_accounts) % len(rpcs)],
                        batch_sync,
                        mode,
                    )
                )
                for j, raw in enumerate(chunk)
            ]
            await asyncio.gather(*tasks)
            if i + batch_size < len(txs) and batch_interval > 0:
                await asyncio.sleep(batch_interval)
