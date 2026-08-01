from time import sleep, time

import pytest

from .state_safety import check_app_hash_agreement, historical_query_soak, open_fd_count

# Small enough that a devnet a few seconds old already satisfies it: the leak
# reproduces at any height below the current one, so a deep lookback buys
# nothing and only makes the test skip itself on a freshly-started chain.
HISTORICAL_LOOKBACK = 5
# One leaked fd per query. Exhaustion errors alone are an unreliable signal —
# the soft ulimit is often far above this count — so the fd delta below is what
# actually catches a leak; the iteration count only has to dwarf the tolerance.
SOAK_ITERATIONS = 1200
# Absorbs unrelated churn in the node's fd table during the soak (new RPC
# connections, WAL/sst rotation), while staying far below SOAK_ITERATIONS.
FD_GROWTH_TOLERANCE = 64
HEIGHT_WAIT_TIMEOUT = 60
# Forward samples to observe: each node has to commit this many further blocks
# while its own computed app hash is polled.
APP_HASH_BLOCKS = 3


@pytest.mark.rpc_diff
def test_app_hash_agreement(devnet):
    # Deliberately no height window: a node only exposes the app hash it computed
    # for its current tip, and a window clamped to the slowest node would shrink
    # to accommodate a halted one instead of failing on it.
    assert check_app_hash_agreement(devnet.nodes, blocks=APP_HASH_BLOCKS) == []


def _wait_for_height(w3, height: int) -> None:
    """The smoke script only waits for the JSON-RPC port, so the chain can still
    be at height 1 when this runs."""
    deadline = time() + HEIGHT_WAIT_TIMEOUT
    while w3.eth.block_number < height:
        assert time() < deadline, (
            f"chain stalled at height {w3.eth.block_number} after "
            f"{HEIGHT_WAIT_TIMEOUT}s, needs {height} for a historical query soak"
        )
        sleep(1)


def test_historical_query_soak(devnet, funded_account):
    """Detects the memiavl historical-query fd leak by the node's fd delta across
    the soak. Against a remote devnet the fd table isn't reachable, and then this
    only catches a leak severe enough to make queries themselves fail."""
    node = devnet.nodes[0]
    w3 = node.w3
    _wait_for_height(w3, HISTORICAL_LOOKBACK + 1)
    height = w3.eth.block_number - HISTORICAL_LOOKBACK

    # CometBFT RPC and JSON-RPC are served by the same cronosd process.
    fds_before = open_fd_count(node.rpc)
    result = historical_query_soak(w3, funded_account.address, height, SOAK_ITERATIONS)
    assert result.errors == []

    fds_after = open_fd_count(node.rpc)
    if fds_before is None or fds_after is None:
        pytest.skip(
            f"{node.name} is not a local process — fd growth unobservable, "
            "only query errors were checked"
        )
    assert fds_after - fds_before <= FD_GROWTH_TOLERANCE, (
        f"{node.name} open fds grew {fds_before} -> {fds_after} over "
        f"{SOAK_ITERATIONS} historical queries"
    )
