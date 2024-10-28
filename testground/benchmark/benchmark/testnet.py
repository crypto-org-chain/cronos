import asyncio
import json
import sys
import time
from pathlib import Path

import click
import requests
import web3
from hexbytes import HexBytes

from .stats import dump_block_stats
from .transaction import EthTx, build_cosmos_tx, gen, json_rpc_send_body, send
from .utils import block_height, gen_account, split_batch

# arbitrarily picked for testnet, to not conflict with devnet benchmark accounts.
GLOBAL_SEQ = 999
GAS_PRICE = 5050000000000
CHAIN_ID = 338
TESTNET_JSONRPC = "https://evm-t3.cronos.org"
TESTNET_RPC = "https://rpc-t3.cronos.org"
TESTNET_EVM_DENOM = "basetcro"


@click.group()
def cli():
    pass


@cli.command()
@click.option("--json-rpc", default=TESTNET_JSONRPC)
@click.option("--rpc", default=TESTNET_RPC)
@click.option("--batch-size", default=200)
@click.argument("start", type=int)
@click.argument("end", type=int)
def fund(json_rpc, rpc, batch_size, start, end):
    w3 = web3.Web3(web3.HTTPProvider(json_rpc))
    fund_account = gen_account(GLOBAL_SEQ, 0)
    fund_address = HexBytes(fund_account.address)
    nonce = w3.eth.get_transaction_count(fund_account.address)

    batches = split_batch(end - start + 1, batch_size)
    for begin, end in batches:
        begin += start
        end += start
        txs = []
        for i in range(begin, end):
            tx = {
                "to": gen_account(GLOBAL_SEQ, i).address,
                "value": 10 * 10**18,
                "nonce": nonce,
                "gas": 21000,
                "gasPrice": GAS_PRICE,
                "chainId": CHAIN_ID,
            }
            txs.append(
                EthTx(
                    tx, fund_account.sign_transaction(tx).rawTransaction, fund_address
                )
            )
            nonce += 1
        raw = build_cosmos_tx(*txs, msg_version="1.3", evm_denom=TESTNET_EVM_DENOM)
        rsp = requests.post(
            rpc, json=json_rpc_send_body(raw, method="broadcast_tx_sync")
        ).json()
        if rsp["result"]["code"] != 0:
            print(rsp["result"]["log"])
            break

        # wait for nonce to change
        while True:
            if w3.eth.get_transaction_count(fund_account.address) >= nonce:
                break
            time.sleep(1)

        print("sent", begin, end)


@cli.command()
@click.option("--json-rpc", default=TESTNET_JSONRPC)
@click.argument("start", type=int)
@click.argument("end", type=int)
def check(json_rpc, start, end):
    w3 = web3.Web3(web3.HTTPProvider(json_rpc))
    for i in range(start, end + 1):
        addr = gen_account(GLOBAL_SEQ, i).address
        nonce = w3.eth.get_transaction_count(addr)
        balance = int(w3.eth.get_balance(addr))
        print(i, addr, nonce, balance)


@cli.command()
@click.argument("start", type=int)
@click.argument("end", type=int)
@click.option("--num-txs", default=1)
@click.option("--nonce", default=0)
@click.option("--msg-version", default="1.3")
def gen_txs(start, end, num_txs, nonce, msg_version):
    num_accounts = end - start + 1
    txs = gen(
        GLOBAL_SEQ,
        num_accounts,
        num_txs,
        "simple-transfer",
        1,
        start_account=start,
        nonce=nonce,
        msg_version=msg_version,
        tx_options={"gas_price": GAS_PRICE, "chain_id": CHAIN_ID},
        evm_denom=TESTNET_EVM_DENOM,
    )
    json.dump(txs, sys.stdout)


@cli.command()
@click.argument("path", type=str)
@click.option("--rpc", default=TESTNET_RPC)
@click.option("--sync/--async", default=False)
def send_txs(path, rpc, sync):
    txs = json.loads(Path(path).read_text())
    asyncio.run(send(txs, rpc, sync))


@cli.command()
@click.option("--json-rpc", default=TESTNET_JSONRPC)
@click.option("--rpc", default=TESTNET_RPC)
@click.option("--count", default=30)
def stats(json_rpc, rpc, count):
    current = block_height(rpc)
    dump_block_stats(
        sys.stdout,
        eth=True,
        rpc=rpc,
        json_rpc=json_rpc,
        start=current - count,
        end=current,
    )


if __name__ == "__main__":
    cli()
