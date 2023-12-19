from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import pytest
from .network import setup_custom_cronos
from .utils import ADDRS, KEYS, send_transaction, w3_wait_for_block



@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("min-gas-price")
    yield from setup_custom_cronos(
        path, 26500, Path(__file__).parent / "configs/min_gas_price.jsonnet"
    )


def adjust_base_fee(parent_fee, gas_limit, gas_used, params):
    "spec: https://eips.ethereum.org/EIPS/eip-1559#specification"
    change_denominator = params["base_fee_change_denominator"]
    elasticity_multiplier = params["elasticity_multiplier"]
    gas_target = gas_limit // elasticity_multiplier

    delta = parent_fee * (gas_target - gas_used) // gas_target // change_denominator
    # https://github.com/crypto-org-chain/ethermint/blob/develop/x/feemarket/keeper/eip1559.go#L104
    return max(parent_fee - delta, params["min_gas_price"])


def exec(cronos, custom_cronos, geth, process):
    providers = [cronos.w3, custom_cronos.w3, geth.w3]
    with ThreadPoolExecutor(len(providers)) as exec:
        tasks = [exec.submit(process, w3) for w3 in providers]
        [future.result() for future in as_completed(tasks)]


def get_params(w3, cronos, custom_cronos, geth):
    if w3 == geth.w3:
        return {
            "base_fee_change_denominator": 8,
            "elasticity_multiplier": 2,
            "min_gas_price": 0,
        }
    cli = cronos.cosmos_cli() if w3 == cronos.w3 else custom_cronos.cosmos_cli()
    params = cli.query_params("feemarket")["params"]
    return {k: int(float(v)) for k, v in params.items()}


def test_dynamic_fee_tx(cronos, custom_cronos, geth):
    """
    test basic eip-1559 tx works:
    - tx fee calculation is compliant to go-ethereum
    - base fee adjustment is compliant to go-ethereum
    """

    def process(w3):
        amount = 10000
        before = w3.eth.get_balance(ADDRS["community"])
        tip_price = 1
        max_price = 1000000000000 + tip_price
        if custom_cronos.w3 == w3:
            max_price = 10000000000000 + tip_price
        tx = {
            "to": "0x0000000000000000000000000000000000000000",
            "value": amount,
            "gas": 21000,
            "maxFeePerGas": max_price,
            "maxPriorityFeePerGas": tip_price,
        }
        txreceipt = send_transaction(w3, tx, KEYS["community"])
        assert txreceipt.status == 1
        blk = w3.eth.get_block(txreceipt.blockNumber)
        assert txreceipt.effectiveGasPrice == blk.baseFeePerGas + tip_price

        fee_expected = txreceipt.gasUsed * txreceipt.effectiveGasPrice
        after = w3.eth.get_balance(ADDRS["community"])
        fee_deducted = before - after - amount
        assert fee_deducted == fee_expected

        assert blk.gasUsed == txreceipt.gasUsed  # we are the only tx in the block

        # check the next block's base fee is adjusted accordingly
        w3_wait_for_block(w3, txreceipt.blockNumber + 1)
        fee = w3.eth.get_block(txreceipt.blockNumber + 1).baseFeePerGas
        params = get_params(w3, cronos, custom_cronos, geth)
        assert fee == adjust_base_fee(
            blk.baseFeePerGas, blk.gasLimit, blk.gasUsed, params
        ), fee

    exec(cronos, custom_cronos, geth, process)


def test_base_fee_adjustment(cronos, custom_cronos, geth):
    """
    verify base fee adjustment of three continuous empty blocks
    """

    def process(w3):
        begin = w3.eth.block_number
        w3_wait_for_block(w3, begin + 3)

        blk = w3.eth.get_block(begin)
        parent_fee = blk.baseFeePerGas
        params = get_params(w3, cronos, custom_cronos, geth)

        for i in range(3):
            fee = w3.eth.get_block(begin + 1 + i).baseFeePerGas
            assert fee == adjust_base_fee(parent_fee, blk.gasLimit, 0, params)
            parent_fee = fee

    exec(cronos, custom_cronos, geth, process)


def test_recommended_fee_per_gas(cronos, custom_cronos, geth):
    """The recommended base fee per gas returned by eth_gasPrice is
    base fee of the block just produced + eth_maxPriorityFeePerGas (the buffer).\n
    Verify the calculation of recommended base fee per gas (eth_gasPrice)
    """

    def process(w3):
        recommended_base_fee_per_gas = w3.eth.gas_price
        latest_block = w3.eth.get_block("latest")
        base_fee = latest_block["baseFeePerGas"]
        buffer_fee = w3.eth.max_priority_fee

        assert recommended_base_fee_per_gas == base_fee + buffer_fee, (
            f"eth_gasPrice is not the {latest_block['number']} block's "
            "base fee plus eth_maxPriorityFeePerGas"
        )

    exec(cronos, custom_cronos, geth, process)


def test_current_fee_per_gas_should_not_be_smaller_than_next_block_base_fee(
    cronos,
    custom_cronos,
    geth,
):
    """The recommended base fee per gas returned by eth_gasPrice should
    be bigger than or equal to the base fee per gas of the next block, \n
    otherwise the tx does not meet the requirement to be included in the next block.\n
    """

    def process(w3):
        base_block = w3.eth.block_number
        recommended_base_fee = w3.eth.gas_price

        w3_wait_for_block(w3, base_block + 1)
        next_block = w3.eth.get_block(base_block + 1)
        assert recommended_base_fee >= next_block["baseFeePerGas"], (
            f"recommended base fee: {recommended_base_fee} is smaller than "
            f"next block {next_block['number']} base fee: {next_block['baseFeePerGas']}"
        )

    exec(cronos, custom_cronos, geth, process)
