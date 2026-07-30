from remote_benchmark.contracts import (
    NFT_ADDRESS,
    NFT_BYTECODE,
    POOL_ADDRESS,
    POOL_BYTECODE,
    nft_genesis_account,
    pool_genesis_account,
)


def test_pool_genesis_account_deploys_code_at_the_fixed_address():
    evm, auth = pool_genesis_account(1, 2)

    assert evm["address"] == POOL_ADDRESS
    assert evm["code"] == POOL_BYTECODE
    assert auth["base_account"]["address"]
    assert len(auth["code_hash"]) == 64


def test_pool_genesis_account_packs_reserve0_and_reserve1_into_slot_0():
    reserve0 = 123
    reserve1 = 456
    evm, _auth = pool_genesis_account(reserve0, reserve1)

    assert len(evm["storage"]) == 1
    slot0 = int(evm["storage"][0]["value"], 16)
    assert slot0 & ((1 << 112) - 1) == reserve0
    assert (slot0 >> 112) & ((1 << 112) - 1) == reserve1


def test_nft_genesis_account_deploys_code_with_no_storage_seeding():
    evm, auth = nft_genesis_account()

    assert evm["address"] == NFT_ADDRESS
    assert evm["code"] == NFT_BYTECODE
    assert evm["storage"] == []
    assert len(auth["code_hash"]) == 64
