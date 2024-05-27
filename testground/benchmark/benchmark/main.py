import os
import queue

from pystarport.cosmoscli import ChainCommand

from .context import Context
from .network import get_data_ip


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    print("global_seq:", ctx.global_seq, "group_seq:", ctx.group_seq)

    # broadcast the peer addresses
    addr = get_data_ip(ctx.params)
    peers = ctx.sync.publish_subscribe_simple(
        "peers", str(addr), ctx.params.test_instance_count
    )
    print("peers", peers)

    cmd = ChainCommand("/bin/cronosd")
    print(cmd("version", "--long").decode())

    ctx.record_success()


TEST_CASES = {
    "entrypoint": entrypoint,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
