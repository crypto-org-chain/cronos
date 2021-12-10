from .utils import ADDRS, KEYS, send_transaction, w3_wait_for_block


def adjust_base_fee(parent_fee, gas_limit, gas_used):
    "spec: https://eips.ethereum.org/EIPS/eip-1559#specification"
    change_denominator = 8
    elasticity_multiplier = 2
    gas_target = gas_limit // elasticity_multiplier

    delta = parent_fee * (gas_target - gas_used) // gas_target // change_denominator
    return parent_fee - delta


def test_dynamic_fee_tx(cluster):
    """
    test basic eip-1559 tx works:
    - tx fee calculation is compliant to go-ethereum
    - base fee adjustment is compliant to go-ethereum
    """
    w3 = cluster.w3
    amount = 10000
    before = w3.eth.get_balance(ADDRS["community"])
    tip_price = 1
    max_price = 1000000000000 + tip_price
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
    next_base_price = w3.eth.get_block(txreceipt.blockNumber + 1).baseFeePerGas

    assert next_base_price == adjust_base_fee(
        blk.baseFeePerGas, blk.gasLimit, blk.gasUsed
    )


def test_base_fee_adjustment(cluster):
    """
    verify base fee adjustment of three continuous empty blocks
    """
    w3 = cluster.w3
    begin = w3.eth.block_number
    w3_wait_for_block(w3, begin + 3)

    blk = w3.eth.get_block(begin)
    parent_fee = blk.baseFeePerGas

    for i in range(3):
        fee = w3.eth.get_block(begin + 1 + i).baseFeePerGas
        assert fee == adjust_base_fee(parent_fee, blk.gasLimit, 0)
        parent_fee = fee
