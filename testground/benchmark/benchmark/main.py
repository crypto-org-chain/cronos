import os
import queue

from .context import Context
from .network import get_data_ip


def influxdb_url():
    return os.environ.get("INFLUXDB_URL", "http://testground-influxdb:8086")


def entrypoint(ctx: Context):
    ctx.init_common()

    print("global_seq:", ctx.global_seq, "group_seq:", ctx.group_seq)

    # share the peer addresses
    q = queue.Queue()
    addr = get_data_ip(ctx.params)
    ctx.sync.publish_subscribe("peers", str(addr), q.put)
    peers = {q.get() for i in range(ctx.params.test_instance_count)}
    if len(peers) != ctx.params.test_instance_count:
        ctx.record_failure("peer addresses are not unique")
        ctx.sync.signal_and_wait("failed", ctx.params.test_instance_count)
        return

    print("peers", peers)

    ctx.record_success()


TEST_CASES = {
    "entrypoint": entrypoint,
}


def main():
    with Context() as ctx:
        TEST_CASES[ctx.params.test_case](ctx)


if __name__ == "__main__":
    main()
