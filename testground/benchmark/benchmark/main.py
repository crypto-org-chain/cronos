import json
import os
import subprocess

from .cli import ChainCommand
from .context import Context
from .peer import bootstrap
from .utils import wait_for_port

CRONOSD_PATH = "/bin/cronosd"


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    cli = ChainCommand(CRONOSD_PATH)

    # build the genesis file collectively, and setup the network topology
    bootstrap(ctx, cli)

    # start the node
    proc = subprocess.Popen([CRONOSD_PATH, "start"])

    # wait until halt-height
    wait_for_port(26657)
    while True:
        status = json.loads(cli("status", output="json"))
        height = int(status["sync_info"]["latest_block_height"])

        if height >= ctx.params.halt_height:
            break

    # halt together
    ctx.sync.signal_and_wait("halt", ctx.params.test_instance_count)

    proc.terminate()
    proc.wait(5)
    ctx.record_success()


TEST_CASES = {
    "entrypoint": entrypoint,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
