import os
import subprocess
from pathlib import Path

import web3

from .cli import ChainCommand
from .context import Context
from .peer import CONTAINER_CRONOSD_PATH, bootstrap
from .sendtx import generate_load
from .utils import wait_for_block, wait_for_port


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    cli = ChainCommand(CONTAINER_CRONOSD_PATH)

    # build the genesis file collectively, and setup the network topology
    bootstrap(ctx, cli)

    # start the node
    logfile = Path(ctx.params.test_outputs_path) / "node.log"
    proc = subprocess.Popen(
        [CONTAINER_CRONOSD_PATH, "start"],
        stdout=open(logfile, "ab", buffering=0),
    )

    wait_for_port(26657)
    wait_for_port(8545)
    wait_for_block(cli, 1)

    test_finish_entry = f"finish-test-{ctx.params.test_group_id}"
    if not ctx.is_validator:
        generate_load(cli, ctx.params.num_accounts, ctx.params.num_txs)
        print("finish test", ctx.group_seq)
        ctx.sync.signal_and_wait(
            test_finish_entry, ctx.params.test_group_instance_count
        )

    if ctx.is_fullnode_leader:
        # collect output
        w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
        for i in range(w3.eth.block_number):
            blk = w3.eth.get_block(i)
            print(i, len(blk.transactions), blk.timestamp)

    # halt after all tasks are done
    ctx.sync.signal_and_wait("halt", ctx.params.test_instance_count)

    proc.kill()
    try:
        proc.wait(5)
    except subprocess.TimeoutExpired:
        pass

    ctx.record_success()


def info(ctx: Context):
    """
    Print the runtime configuration, mainly to check if the image is built successfully.
    """
    print(ctx.params)


TEST_CASES = {
    "entrypoint": entrypoint,
    "info": info,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
