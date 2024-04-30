def test_encrypt_decrypt(cronos):
    cli = cronos.cosmos_cli()

    # gen two keys for two accounts
    pubkey0 = cli.keygen(keyring_name="key0")
    cli.register_e2ee_key(pubkey0, _from="validator")
    assert cli.query_e2ee_key(cli.address("validator")) == pubkey0
    pubkey1 = cli.keygen(keyring_name="key1")
    cli.register_e2ee_key(pubkey1, _from="community")

    # query in batch
    assert cli.query_e2ee_keys(cli.address("validator"), cli.address("community")) == [
        pubkey0,
        pubkey1,
    ]

    # prepare data file to encrypt
    content = "Hello World!"
    plainfile = cli.data_dir / "plaintext"
    plainfile.write_text(content)

    cipherfile = cli.data_dir / "ciphertext"
    cli.encrypt(
        plainfile,
        cli.address("validator"),
        cli.address("community"),
        output=cipherfile,
    )

    assert cli.decrypt(cipherfile, identity="key0") == content
    assert cli.decrypt(cipherfile, identity="key1") == content
