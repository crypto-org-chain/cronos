from pathlib import Path

import pytest
from eth_utils import to_checksum_address
from pystarport import ports

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    bech32_to_eth,
    encrypt_to_validators,
    gen_validator_identity,
    get_unconfirmed_txs,
    wait_for_new_blocks,
)


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("more_validators")
    yield from setup_custom_cronos(
        path, 26800, Path(__file__).parent / "configs/more_validators.jsonnet"
    )


def test_block_list_evm(custom_cronos):
    gen_validator_identity(custom_cronos)
    cli = custom_cronos.cosmos_cli()
    user = cli.address("signer2")
    # set blocklist
    encrypt_to_validators(cli, {"addresses": [user]})
    u = to_checksum_address(bech32_to_eth(user))
    w3 = custom_cronos.w3
    n = w3.eth.get_transaction_count(u)
    tx = {
        "from": u,
        "to": ADDRS["community"],
        "value": 1,
    }
    tx0 = tx | {"nonce": n}
    tx1 = tx | {"nonce": n + 1}
    base_port = custom_cronos.base_port(0)
    p = ports.rpc_port(base_port)
    assert not get_unconfirmed_txs(p)
    txhash = w3.eth.send_transaction(tx0).hex()
    txhash1 = w3.eth.send_transaction(tx1).hex()
    nonce = int(cli.query_account(user)["base_account"]["sequence"])

    r = get_unconfirmed_txs(p)
    assert len(r) == 2
    # clear blocklist
    encrypt_to_validators(cli, {})

    # the blocked tx should be unblocked now
    wait_for_new_blocks(cli, 1)
    last = int(cli.query_account(user)["base_account"]["sequence"])
    assert nonce + 2 == last
    assert not get_unconfirmed_txs(p)

    for txhash in [txhash, txhash1]:
        assert len(cli.tx_search(f"ethereum_tx.ethereumTxHash='{txhash}'")["txs"]) == 1
