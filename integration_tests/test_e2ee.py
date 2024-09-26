import json

import pytest
from eth_utils import to_checksum_address
from hexbytes import HexBytes
from pystarport import ports

from .network import Cronos
from .utils import ADDRS, bech32_to_eth, wait_for_new_blocks, wait_for_port


def test_register(cronos: Cronos):
    cli = cronos.cosmos_cli()
    pubkey0 = cli.e2ee_keygen(keyring_name="key0")
    with pytest.raises(AssertionError) as exc:
        cli.register_e2ee_key(pubkey0 + "malformed", _from="validator")
    assert "malformed recipient" in str(exc.value)
    assert not cli.query_e2ee_key(cli.address("validator"))


def gen_validator_identity(cronos: Cronos):
    for i in range(len(cronos.config["validators"])):
        cli = cronos.cosmos_cli(i)
        if cli.query_e2ee_key(cli.address("validator")):
            return
        pubkey = cli.e2ee_keygen()
        assert cli.e2ee_pubkey() == pubkey
        cli.register_e2ee_key(pubkey, _from="validator")
        assert cli.query_e2ee_key(cli.address("validator")) == pubkey

        cronos.supervisorctl("restart", f"cronos_777-1-node{i}")

    wait_for_new_blocks(cronos.cosmos_cli(), 1)


def test_encrypt_decrypt(cronos):
    gen_validator_identity(cronos)

    cli0 = cronos.cosmos_cli()
    cli1 = cronos.cosmos_cli(1)

    # query in batch
    assert (
        len(
            cli0.query_e2ee_keys(
                cli0.address("validator"),
                cli1.address("validator"),
            )
        )
        == 2
    )

    # prepare data file to encrypt
    content = "Hello World!"
    plainfile = cli0.data_dir / "plaintext"
    plainfile.write_text(content)
    cipherfile = cli0.data_dir / "ciphertext"
    cli0.e2ee_encrypt(
        plainfile,
        cli0.address("validator"),
        cli1.address("validator"),
        output=cipherfile,
    )

    assert cli0.e2ee_decrypt(cipherfile) == content
    assert cli1.e2ee_decrypt(cipherfile) == content


def encrypt_to_validators(cli, content):
    blocklist = json.dumps(content)
    plainfile = cli.data_dir / "plaintext"
    plainfile.write_text(blocklist)
    cipherfile = cli.data_dir / "ciphertext"
    cli.e2ee_encrypt_to_validators(plainfile, output=cipherfile)
    rsp = cli.store_blocklist(cipherfile, _from="validator")
    assert rsp["code"] == 0, rsp["raw_log"]


def test_block_list(cronos):
    gen_validator_identity(cronos)
    cli = cronos.cosmos_cli()
    user = cli.address("signer2")
    # set blocklist
    encrypt_to_validators(cli, {"addresses": [user]})

    # normal tx works
    cli.transfer(cli.address("validator"), user, "1basetcro")

    # blocked tx can be included into mempool
    rsp = cli.transfer(
        user, cli.address("validator"), "1basetcro", event_query_tx=False
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # but won't be included into block
    txhash = rsp["txhash"]
    with pytest.raises(AssertionError) as exc:
        cli.event_query_tx_for(txhash)
    assert "timed out waiting" in str(exc.value)
    nonce = int(cli.query_account(user)["account"]["value"]["sequence"])

    # clear blocklist
    encrypt_to_validators(cli, {})

    # the blocked tx should be unblocked now
    wait_for_new_blocks(cli, 1)
    assert nonce + 1 == int(cli.query_account(user)["account"]["value"]["sequence"])


def test_block_list_evm(cronos):
    gen_validator_identity(cronos)
    cli = cronos.cosmos_cli()
    user = cli.address("signer2")
    # set blocklist
    encrypt_to_validators(cli, {"addresses": [user]})
    tx = {
        "from": to_checksum_address(bech32_to_eth(user)),
        "to": ADDRS["community"],
        "value": 1,
    }
    base_port = cronos.base_port(0)
    wait_for_port(ports.evmrpc_ws_port(base_port))
    w3 = cronos.w3
    flt = w3.eth.filter("pending")
    assert flt.get_new_entries() == []

    txhash = w3.eth.send_transaction(tx).hex()
    nonce = int(cli.query_account(user)["account"]["value"]["sequence"])
    # check tx in mempool
    assert HexBytes(txhash) in w3.eth.get_filter_changes(flt.filter_id)

    # clear blocklist
    encrypt_to_validators(cli, {})

    # the blocked tx should be unblocked now
    wait_for_new_blocks(cli, 1)
    assert nonce + 1 == int(cli.query_account(user)["account"]["value"]["sequence"])
    assert w3.eth.get_filter_changes(flt.filter_id) == []


def test_invalid_block_list(cronos):
    cli = cronos.cosmos_cli()
    cipherfile = cli.data_dir / "ciphertext"
    cipherfile.write_text("{}")
    with pytest.raises(AssertionError) as exc:
        cli.store_blocklist(cipherfile, _from="validator")
    assert "failed to read header" in str(exc.value)
