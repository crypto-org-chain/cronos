import json

import pytest

from .network import Cronos
from .utils import wait_for_new_blocks


def test_register(cronos: Cronos):
    cli = cronos.cosmos_cli()
    pubkey0 = cli.keygen(keyring_name="key0")
    with pytest.raises(AssertionError) as exc:
        cli.register_e2ee_key(pubkey0 + "malformed", _from="validator")
    assert "malformed recipient" in str(exc.value)
    assert not cli.query_e2ee_key(cli.address("validator"))


def gen_validator_identity(cronos: Cronos):
    for i in range(len(cronos.config["validators"])):
        cli = cronos.cosmos_cli(i)
        if cli.query_e2ee_key(cli.address("validator")):
            return
        pubkey = cli.keygen()
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
    cli0.encrypt(
        plainfile,
        cli0.address("validator"),
        cli1.address("validator"),
        output=cipherfile,
    )

    assert cli0.decrypt(cipherfile) == content
    assert cli1.decrypt(cipherfile) == content


def encrypt_to_validators(cli, content):
    blocklist = json.dumps(content)
    plainfile = cli.data_dir / "plaintext"
    plainfile.write_text(blocklist)
    cipherfile = cli.data_dir / "ciphertext"
    cli.encrypt_to_validators(plainfile, output=cipherfile)
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

    nonce = int(cli.query_account(user)["base_account"]["sequence"])

    # clear blocklist
    encrypt_to_validators(cli, {})

    # the blocked tx should be unblocked now
    wait_for_new_blocks(cli, 1)
    assert nonce + 1 == int(cli.query_account(user)["base_account"]["sequence"])
