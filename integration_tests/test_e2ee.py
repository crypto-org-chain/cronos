import json

from .utils import prepare_cipherfile, wait_for_new_blocks


def test_encrypt_decrypt(cronos):
    cli = cronos.cosmos_cli()
    cli1 = cronos.cosmos_cli(1)
    name0 = "key0"
    name1 = "key1"
    content = "Hello World!"
    cipherfile = prepare_cipherfile(cli, cli1, name0, name1, content)
    assert cli.decrypt(cipherfile, identity=name0) == content
    assert cli1.decrypt(cipherfile, identity=name1) == content


def test_block_list(cronos):
    cli0 = cronos.cosmos_cli()
    cli1 = cronos.cosmos_cli(1)

    # prepare encryption keys for validators
    cli0.register_e2ee_key(cli0.keygen(), _from="validator")
    cli1.register_e2ee_key(cli1.keygen(), _from="validator")

    user = cli0.address("user")

    blocklist = json.dumps({"addresses": [user]})
    plainfile = cli0.data_dir / "plaintext"
    plainfile.write_text(blocklist)
    cipherfile = cli0.data_dir / "ciphertext"
    cli0.encrypt_to_validators(plainfile, output=cipherfile)
    rsp = cli0.store_blocklist(cipherfile, _from="validator")
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cli0, 2)
    rsp = cli0.transfer(user, cli0.address("validator"), "1basetcro")
    assert rsp["code"] != 0
