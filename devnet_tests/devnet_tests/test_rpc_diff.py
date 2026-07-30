import pytest

from .rpc_diff import run_rpc_diff

EQUIVALENCE_THRESHOLD = 0.90


@pytest.mark.rpc_diff
def test_rpc_diff_equivalence(devnet):
    latest = devnet.nodes[0].w3.eth.block_number
    start = max(latest - 5, 0)
    report = run_rpc_diff(devnet, start, latest)

    if report.compared == 0:
        pytest.skip("no comparable responses in the sampled height range")

    assert report.equivalence_rate >= EQUIVALENCE_THRESHOLD, (
        f"equivalence rate {report.equivalence_rate:.2%} below "
        f"{EQUIVALENCE_THRESHOLD:.0%}: {report.to_dict()['mismatches']}"
    )
