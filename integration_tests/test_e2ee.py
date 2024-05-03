def test_encrypt_decrypt(cronos):
    cli0 = cronos.cosmos_cli()
    cli1 = cronos.cosmos_cli(1)

    # gen two keys for two accounts
    name0 = "key0"
    name1 = "key1"
    pubkey0 = cli0.keygen(keyring_name=name0)
    pubkey1 = cli1.keygen(keyring_name=name1)
    sender = "validator"
    cli0.register_e2ee_key(pubkey0, _from=sender)
    cli1.register_e2ee_key(pubkey1, _from=sender)
    # query in batch
    assert cli0.query_e2ee_keys(cli0.address(sender), cli1.address(sender)) == [
        pubkey0,
        pubkey1,
    ]
    # prepare data file to encrypt
    content = "Hello World!"
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
