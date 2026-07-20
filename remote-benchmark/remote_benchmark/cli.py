import asyncio
import itertools
import sys
import time
from pathlib import Path

import click
import requests
import ujson
import web3
from hexbytes import HexBytes

from .config import load_config
from .monitor import BlockSTMMonitor, MempoolMonitor
from .stats import (
    _fetch_prometheus,
    dump_block_stats,
    dump_eth_block_stats,
    scrape_consensus_raw,
)
from .transaction import (
    EthTx,
    build_cosmos_tx,
    gen,
    json_rpc_send_body,
    send_round_robin,
)
from .utils import block_height, block_txs, eth_block_number, gen_account, split_batch

# reserved for the funding account, index 0 is the funder itself.
FUND_ACCOUNT_INDEX = 0
LOAD_COMMIT_TIMEOUT = 120


def wait_for_committed_txs(rpc, start, end, expected_txs, timeout=LOAD_COMMIT_TIMEOUT):
    """Extend the sample until all generated Cosmos txs are committed."""
    # ``start`` is the pre-send anchor and can still contain setup traffic,
    # so only count envelopes committed after it.
    next_height = start + 1
    committed_txs = 0
    deadline = time.monotonic() + timeout

    while True:
        while next_height <= end:
            committed_txs += len(block_txs(next_height, rpc) or [])
            next_height += 1
            if committed_txs >= expected_txs:
                return end, committed_txs

        if time.monotonic() >= deadline:
            return end, committed_txs

        current = block_height(rpc)
        if current > end:
            end = current
        else:
            time.sleep(0.2)


@click.group()
def cli():
    pass


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--batch-size", default=200)
@click.option("--mode", "fund_mode", type=click.Choice(["cosmos", "eth"]))
@click.argument("start", type=int)
@click.argument("end", type=int)
def fund(config_path, batch_size, fund_mode, start, end):
    """Fund generated test accounts [start, end] from the funding account."""
    cfg = load_config(config_path)
    fund_mode = fund_mode or cfg.mode
    w3 = web3.Web3(web3.HTTPProvider(cfg.primary.json_rpc))
    fund_account = gen_account(cfg.global_seq, FUND_ACCOUNT_INDEX)
    fund_address = HexBytes(fund_account.address)
    nonce = w3.eth.get_transaction_count(fund_account.address)

    for begin, chunk_end in split_batch(end - start + 1, batch_size):
        begin += start
        chunk_end += start
        txs = []
        for i in range(begin, chunk_end):
            tx = {
                "to": gen_account(cfg.global_seq, i).address,
                # 50 CRO/account: headroom over the worst-case batch configs'
                # 200 txs x 21000 gas x 5e12 gas_price = 21 CRO in ante-handler
                # fees per account - too little here means every batched
                # MsgEthereumTx past the point funds run out silently fails
                # CheckTx (insufficient funds), and the benchmark reports a
                # no_load_period with no error surfaced anywhere.
                "value": 50 * 10**18,
                "nonce": nonce,
                "gas": 21000,
                "gasPrice": cfg.gas_price,
                "chainId": cfg.chain_id,
            }
            txs.append(
                EthTx(
                    tx, fund_account.sign_transaction(tx).rawTransaction, fund_address
                )
            )
            nonce += 1

        if fund_mode == "eth":
            for tx in txs:
                w3.eth.send_raw_transaction(tx.raw)
        else:
            raw = build_cosmos_tx(
                *txs, msg_version=cfg.msg_version, evm_denom=cfg.evm_denom
            )
            rsp = requests.post(
                cfg.primary.rpc,
                json=json_rpc_send_body(raw, method="broadcast_tx_sync"),
            ).json()
            if rsp["result"]["code"] != 0:
                print(rsp["result"]["log"])
                break

        # wait for nonce to change
        while w3.eth.get_transaction_count(fund_account.address) < nonce:
            time.sleep(1)

        print("sent", begin, chunk_end)


@cli.command()
@click.option("--config", "config_path", required=True)
@click.argument("start", type=int)
@click.argument("end", type=int)
def check(config_path, start, end):
    """Query nonce/balance for generated test accounts [start, end]."""
    cfg = load_config(config_path)
    json_rpcs = itertools.cycle(cfg.json_rpcs)
    for i in range(start, end + 1):
        w3 = web3.Web3(web3.HTTPProvider(next(json_rpcs)))
        addr = gen_account(cfg.global_seq, i).address
        nonce = w3.eth.get_transaction_count(addr)
        balance = int(w3.eth.get_balance(addr))
        print(i, addr, nonce, balance)


@cli.command("gen-txs")
@click.option("--config", "config_path", required=True)
@click.option("--nonce", default=0)
@click.option("--start-account", default=0)
@click.option("-o", "--output", "output_path", default=None, help="default: stdout")
@click.argument("start", type=int)
@click.argument("end", type=int)
def gen_txs(config_path, nonce, start_account, output_path, start, end):
    """Generate a signed tx batch for accounts [start, end]."""
    cfg = load_config(config_path)
    num_accounts = end - start + 1
    txs = gen(
        cfg.global_seq,
        num_accounts,
        cfg.num_txs,
        cfg.tx_type,
        cfg.batch_size,
        start_account=start + start_account,
        nonce=nonce,
        msg_version=cfg.msg_version,
        tx_options={"gas_price": cfg.gas_price, "chain_id": cfg.chain_id},
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
    )
    print(
        f"generated {num_accounts * cfg.num_txs} EVM txs "
        f"in {len(txs)} {cfg.mode} txs",
        file=sys.stderr,
    )
    payload = {"num_accounts": num_accounts, "txs": txs}
    if output_path:
        Path(output_path).write_text(ujson.dumps(payload))
    else:
        ujson.dump(payload, sys.stdout)


@cli.command("send-txs")
@click.option("--config", "config_path", required=True)
@click.option("--sync/--async", default=False)
@click.argument("path", type=str)
def send_txs(config_path, sync, path):
    """Round-robin broadcast a tx batch file across all configured endpoints."""
    cfg = load_config(config_path)
    payload = ujson.loads(Path(path).read_text())
    txs = payload["txs"]
    num_accounts = payload["num_accounts"]
    rpcs = cfg.json_rpcs if cfg.mode == "eth" else cfg.rpcs
    asyncio.run(
        send_round_robin(
            txs,
            rpcs,
            sync=sync,
            batch_size=cfg.send_batch_size,
            batch_interval=cfg.send_interval,
            mode=cfg.mode,
            num_accounts=num_accounts,
        )
    )


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--count", default=30)
def stats(config_path, count):
    """Dump block/TPS/gas stats from the primary endpoint."""
    cfg = load_config(config_path)
    if cfg.mode == "eth":
        current = eth_block_number(cfg.primary.json_rpc)
        dump_eth_block_stats(
            sys.stdout,
            json_rpc=cfg.primary.json_rpc,
            start=max(2, current - count),
            end=current,
        )
        return

    current = block_height(cfg.primary.rpc)
    dump_block_stats(
        sys.stdout,
        rpc=cfg.primary.rpc,
        json_rpc=cfg.primary.json_rpc,
        telemetry=cfg.telemetry,
        start=max(2, current - count),
        end=current,
    )


@cli.command()
@click.option("--config", "config_path", required=True)
@click.option("--nonce", default=0)
@click.option(
    "--probe-batches",
    default=1,
    help=(
        "Send this many leading batches synchronously so CheckTx rejections "
        "surface immediately, instead of the silent no-op you get from "
        "broadcast_tx_async when every tx is rejected. Set to 0 to disable."
    ),
)
@click.argument("start", type=int)
@click.argument("end", type=int)
def bench(config_path, nonce, probe_batches, start, end):
    """Generate load, send it round-robin across all endpoints, then report stats."""
    cfg = load_config(config_path)
    num_accounts = end - start + 1

    print("generating txs...", file=sys.stderr)
    txs = gen(
        cfg.global_seq,
        num_accounts,
        cfg.num_txs,
        cfg.tx_type,
        cfg.batch_size,
        start_account=start,
        nonce=nonce,
        msg_version=cfg.msg_version,
        tx_options={"gas_price": cfg.gas_price, "chain_id": cfg.chain_id},
        evm_denom=cfg.evm_denom,
        wire_format=cfg.mode,
    )
    print(
        f"generated {num_accounts * cfg.num_txs} EVM txs "
        f"in {len(txs)} {cfg.mode} txs",
        file=sys.stderr,
    )

    if cfg.mode == "eth":
        load_start = eth_block_number(cfg.primary.json_rpc)
        print("sending txs...", file=sys.stderr)
        asyncio.run(
            send_round_robin(
                txs,
                cfg.json_rpcs,
                batch_size=cfg.send_batch_size,
                batch_interval=cfg.send_interval,
                mode=cfg.mode,
                num_accounts=num_accounts,
                probe_batches=probe_batches,
            )
        )
        load_end = eth_block_number(cfg.primary.json_rpc)
        dump_eth_block_stats(
            sys.stdout,
            json_rpc=cfg.primary.json_rpc,
            start=load_start,
            end=load_end,
        )
        return

    mempool_monitor = MempoolMonitor(cfg.primary.rpc)
    stm_monitor = BlockSTMMonitor(cfg.primary.rpc, cfg.telemetry)
    consensus_baseline = scrape_consensus_raw(_fetch_prometheus(cfg.telemetry))

    load_start = block_height(cfg.primary.rpc)
    mempool_monitor.start()
    stm_monitor.start()
    committed_txs = 0
    try:
        print("sending txs...", file=sys.stderr)
        asyncio.run(
            send_round_robin(
                txs,
                cfg.rpcs,
                batch_size=cfg.send_batch_size,
                batch_interval=cfg.send_interval,
                num_accounts=num_accounts,
                probe_batches=probe_batches,
            )
        )
        load_end = block_height(cfg.primary.rpc)
        load_end, committed_txs = wait_for_committed_txs(
            cfg.primary.rpc, load_start, load_end, len(txs)
        )
    finally:
        mempool_monitor.stop()
        stm_monitor.stop()

    dump_block_stats(
        sys.stdout,
        rpc=cfg.primary.rpc,
        json_rpc=cfg.primary.json_rpc,
        telemetry=cfg.telemetry,
        start=load_start,
        end=load_end,
        mempool_data=mempool_monitor.data,
        stm_data=stm_monitor.data,
        consensus_baseline=consensus_baseline,
    )
    print(f"committed_cosmos_txs {committed_txs}/{len(txs)}")
    if committed_txs < len(txs):
        raise click.ClickException(
            f"timed out waiting for generated transactions to commit: "
            f"{committed_txs}/{len(txs)} Cosmos transactions committed"
        )


if __name__ == "__main__":
    cli()
