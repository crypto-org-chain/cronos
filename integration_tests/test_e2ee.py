import json

import pytest

from .network import Cronos
from .utils import wait_for_new_blocks


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


def test_block_list(cronos):
    gen_validator_identity(cronos)
    cli = cronos.cosmos_cli()

    user = cli.address("user")

    blocklist = json.dumps({"addresses": [user]})
    plainfile = cli.data_dir / "plaintext"
    plainfile.write_text(blocklist)
    cipherfile = cli.data_dir / "ciphertext"
    cli.encrypt_to_validators(plainfile, output=cipherfile)
    rsp = cli.store_blocklist(cipherfile, _from="validator")
    assert rsp["code"] == 0, rsp["raw_log"]

    # normal tx works
    cli.transfer(cli.address("validator"), user, "1basetcro")

    # blocked tx don't work
    with pytest.raises(AssertionError) as exc:
        cli.transfer(user, cli.address("validator"), "1basetcro")

    assert "timed out waiting for event" in str(exc.value)
