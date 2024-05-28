import os
import subprocess

from .cli import ChainCommand
from .context import Context
from .peer import bootstrap

CRONOSD_PATH = "/bin/cronosd"


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    cli = ChainCommand(CRONOSD_PATH)

    # build the genesis file collectively, and setup the network topology
    bootstrap(ctx, cli)

    # start the node
    with subprocess.Popen([CRONOSD_PATH, "start", "--halt-height", "10"]) as proc:
        proc.wait()

    ctx.record_success()


TEST_CASES = {
    "entrypoint": entrypoint,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
