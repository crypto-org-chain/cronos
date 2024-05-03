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
    plainfile = cli0.data_dir / "plaintext"
    plainfile.write_text(content)
    cipherfile = cli0.data_dir / "ciphertext"
    cli0.encrypt(
        plainfile,
        cli0.address(sender),
        cli1.address(sender),
        output=cipherfile,
    )

    assert cli0.decrypt(cipherfile, identity="key0") == content
    assert cli1.decrypt(cipherfile, identity="key1") == content
