import os

from pystarport.cosmoscli import ChainCommand

from .context import Context
from .peer import bootstrap


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    print("params", ctx.params)
    print("global_seq:", ctx.global_seq, "group_seq:", ctx.group_seq)

    # broadcast the peer information
    cli = ChainCommand("/bin/cronosd")
    peers, genesis = bootstrap(ctx, cli)
    print("peers", len(peers))

    ctx.record_success()


TEST_CASES = {
    "entrypoint": entrypoint,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
