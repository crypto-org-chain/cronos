from time import sleep, time

import pytest

from .mempool_probes import CREATE, _sign_and_send
from .rpc_diff import run_rpc_diff

SAMPLE_WINDOW = 5
NODE_CATCHUP_TIMEOUT = 60
# Every method here needs a tx hash from the sampled block, so all of them go
# silently uncompared on an idle chain's empty blocks.
TX_HASH_METHODS = {
    "eth_getTransactionByHash",
    "eth_getTransactionReceipt",
    "debug_traceTransaction",
}

# Init code returning the 10-byte runtime `602a60005260206000f3`, which stores 42
# and returns it for any calldata. Deployed so the `call` category has real
# bytecode to execute instead of degrading to a no-op against an EOA.
STORE_42_INIT_CODE = "0x69602a60005260206000f3600052600a6016f3"
DEPLOY_GAS = 200000
CALL_GAS = 100000
# Non-empty so build_context's contract-call scan recognises the tx; the runtime
# ignores its calldata.
PROBE_CALLDATA = "0x2a"


def _wait_for_all_nodes(devnet, height: int) -> None:
    """run_rpc_diff clamps the range to the least-caught-up node, which would
    otherwise drop the one height that has tx data."""
    deadline = time() + NODE_CATCHUP_TIMEOUT
    for node in devnet.nodes:
        while node.w3.eth.block_number < height:
            assert time() < deadline, (
                f"{node.name} did not reach height {height} within "
                f"{NODE_CATCHUP_TIMEOUT}s"
            )
            sleep(0.5)


def _land_contract_call(devnet, account) -> int:
    """Deploy a tiny contract, then call it, and return the call's height once
    every node has reached it."""
    w3 = devnet.nodes[0].w3
    gas_price = w3.eth.gas_price
    nonce = w3.eth.get_transaction_count(account.address, "pending")

    deploy_hash = _sign_and_send(
        w3,
        account,
        nonce,
        gas_price,
        to=CREATE,
        data=STORE_42_INIT_CODE,
        gas=DEPLOY_GAS,
    )
    contract = w3.eth.wait_for_transaction_receipt(deploy_hash).contractAddress
    assert contract, "contract creation produced no address"

    call_hash = _sign_and_send(
        w3,
        account,
        nonce + 1,
        gas_price,
        to=contract,
        data=PROBE_CALLDATA,
        gas=CALL_GAS,
    )
    height = w3.eth.wait_for_transaction_receipt(call_hash).blockNumber
    _wait_for_all_nodes(devnet, height)
    return height


@pytest.mark.rpc_diff
def test_rpc_diff_equivalence(devnet, funded_account):
    end = _land_contract_call(devnet, funded_account)
    start = max(end - SAMPLE_WINDOW, 1)
    report = run_rpc_diff(devnet, start, end)

    if report.compared == 0:
        pytest.skip("no comparable responses in the sampled height range")

    # The landed tx makes at least one sampled height usable for the tx-hash
    # methods; a method skipped at every height was never actually diffed and
    # must not count as equivalence.
    heights = end - start + 1
    never_ran = {
        name for name, count in report.skipped_methods.items() if count >= heights
    }
    assert (
        not never_ran & TX_HASH_METHODS
    ), f"never compared at any sampled height: {sorted(never_ran & TX_HASH_METHODS)}"

    # Two nodes erroring identically (a missing namespace, a rejected request)
    # compares equal without exercising anything.
    assert report.never_responded == [], (
        "no node ever returned a real response for: "
        f"{report.never_responded} (errors only)"
    )

    # Without a contract to call, eth_call/eth_estimateGas/eth_createAccessList
    # degrade to a guaranteed-matching no-op against an EOA.
    assert report.contract_call_heights > 0, (
        "no sampled height had a tx calling deployed bytecode — the `call` "
        "category executed no EVM code"
    )

    # Per-method, not an aggregate rate: one method broken at every sampled
    # height is a real regression, and its handful of mismatches would stay
    # inside any tolerance the other passing methods buy.
    assert report.mismatches_by_method == {}, (
        f"methods disagreed across nodes: {report.mismatches_by_method} — "
        f"{report.to_dict()['mismatches']}"
    )
