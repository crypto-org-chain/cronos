import os
import subprocess

from .cli import ChainCommand
from .context import Context
from .peer import bootstrap
from .sendtx import sendtx
from .utils import wait_for_block, wait_for_port

CRONOSD_PATH = "/bin/cronosd"


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    cli = ChainCommand(CRONOSD_PATH)

    # build the genesis file collectively, and setup the network topology
    peer = bootstrap(ctx, cli)

    # start the node
    kwargs = {"stdout": subprocess.DEVNULL}
    if ctx.is_leader:
        del kwargs["stdout"]
    proc = subprocess.Popen([CRONOSD_PATH, "start"], **kwargs)

    wait_for_port(26657)
    wait_for_port(8545)
    wait_for_block(cli, 1)

    if not ctx.is_validator:
        sendtx(cli, peer)

    # halt after all tasks are done
    ctx.sync.signal_and_wait("halt", ctx.params.test_instance_count)

    proc.kill()
    try:
        proc.wait(5)
    except subprocess.TimeoutExpired:
        pass
    ctx.record_success()


TEST_CASES = {
    "entrypoint": entrypoint,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
