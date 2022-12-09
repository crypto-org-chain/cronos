from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import pytest
from web3 import Web3

from .network import setup_custom_cronos
from .utils import (
    ACCOUNTS,
    CONTRACTS,
    deploy_contract,
    send_transaction,
    w3_wait_for_block,
)


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("benchmark")
    yield from setup_custom_cronos(
        path, 26200, Path(__file__).parent / "configs/state_benchmark.jsonnet"
    )


@pytest.mark.benchmark
def test_benchmark_storage(custom_cronos):
    w3: Web3 = custom_cronos.w3
    w3_wait_for_block(w3, 1)
    contract = deploy_contract(w3, CONTRACTS["BenchmarkStorage"])

    n = 3000
    gas = 81500000
    iterations = 200
    parity = 100

    def task(acct, acct_i):
        for i in range(iterations):
            seed = i * 10 + acct_i
            tx = contract.functions.batch_set(seed, n, n * parity).build_transaction(
                {"from": acct.address, "gas": gas}
            )
            print(send_transaction(w3, tx, acct.key))

    accounts = [
        ACCOUNTS["validator"],
        ACCOUNTS["community"],
        ACCOUNTS["signer1"],
        ACCOUNTS["signer2"],
    ]
    with ThreadPoolExecutor(len(accounts)) as exec:
        tasks = [exec.submit(task, acct, i) for i, acct in enumerate(accounts)]
        for t in tasks:
            t.result()
