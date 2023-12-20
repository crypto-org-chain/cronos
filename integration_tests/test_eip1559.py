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


@pytest.fixture(scope="module", params=["cluster", "custom_cronos"])
def custom_cluster(request, cluster, custom_cronos):
    provider = request.param
    if provider == "cluster":
        yield cluster
    elif provider == "custom_cronos":
        yield custom_cronos
    else:
        raise NotImplementedError


def adjust_base_fee(parent_fee, gas_limit, gas_used, params):
    "spec: https://eips.ethereum.org/EIPS/eip-1559#specification"
    change_denominator = params["base_fee_change_denominator"]
    elasticity_multiplier = params["elasticity_multiplier"]
    gas_target = gas_limit // elasticity_multiplier

    delta = parent_fee * (gas_target - gas_used) // gas_target // change_denominator
    # https://github.com/crypto-org-chain/ethermint/blob/develop/x/feemarket/keeper/eip1559.go#L104
    return max(parent_fee - delta, params["min_gas_price"])


def get_params(cli):
    if cli is None:
        return {
            "base_fee_change_denominator": 8,
            "elasticity_multiplier": 2,
            "min_gas_price": 0,
        }
    params = cli.query_params("feemarket")["params"]
    return {k: int(float(v)) for k, v in params.items()}


def test_dynamic_fee_tx(custom_cluster, custom_cronos, geth):
    """
    test basic eip-1559 tx works:
    - tx fee calculation is compliant to go-ethereum
    - base fee adjustment is compliant to go-ethereum
    """
    w3 = custom_cluster.w3
    amount = 10000
    before = w3.eth.get_balance(ADDRS["community"])
    tip_price = 1
    max_price = 1000000000000 + tip_price
    if custom_cluster == custom_cronos:
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
    cli = None if custom_cluster == geth else custom_cluster.cosmos_cli()
    params = get_params(cli)
    assert fee == adjust_base_fee(
        blk.baseFeePerGas, blk.gasLimit, blk.gasUsed, params
    ), fee


def test_base_fee_adjustment(custom_cluster, geth):
    """
    verify base fee adjustment of three continuous empty blocks
    """
    w3 = custom_cluster.w3
    begin = w3.eth.block_number
    w3_wait_for_block(w3, begin + 3)

    blk = w3.eth.get_block(begin)
    parent_fee = blk.baseFeePerGas
    cli = None if custom_cluster == geth else custom_cluster.cosmos_cli()
    params = get_params(cli)

    for i in range(3):
        fee = w3.eth.get_block(begin + 1 + i).baseFeePerGas
        assert fee == adjust_base_fee(parent_fee, blk.gasLimit, 0, params)
        parent_fee = fee


def test_recommended_fee_per_gas(custom_cluster):
    """The recommended base fee per gas returned by eth_gasPrice is
    base fee of the block just produced + eth_maxPriorityFeePerGas (the buffer).\n
    Verify the calculation of recommended base fee per gas (eth_gasPrice)
    """
    w3 = custom_cluster.w3

    recommended_base_fee_per_gas = w3.eth.gas_price
    latest_block = w3.eth.get_block("latest")
    base_fee = latest_block["baseFeePerGas"]
    buffer_fee = w3.eth.max_priority_fee

    assert recommended_base_fee_per_gas == base_fee + buffer_fee, (
        f"eth_gasPrice is not the {latest_block['number']} block's "
        "base fee plus eth_maxPriorityFeePerGas"
    )


def test_current_fee_per_gas_should_not_be_smaller_than_next_block_base_fee(
    custom_cluster,
):
    """The recommended base fee per gas returned by eth_gasPrice should
    be bigger than or equal to the base fee per gas of the next block, \n
    otherwise the tx does not meet the requirement to be included in the next block.\n
    """
    w3 = custom_cluster.w3

    base_block = w3.eth.block_number
    recommended_base_fee = w3.eth.gas_price

    w3_wait_for_block(w3, base_block + 1)
    next_block = w3.eth.get_block(base_block + 1)
    assert recommended_base_fee >= next_block["baseFeePerGas"], (
        f"recommended base fee: {recommended_base_fee} is smaller than "
        f"next block {next_block['number']} base fee: {next_block['baseFeePerGas']}"
    )
