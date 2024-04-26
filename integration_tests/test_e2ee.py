import base64
import tempfile


def test_set_key(cronos):
    cli = cronos.cosmos_cli()
    key = base64.b64encode(b"new_key").decode("utf-8")
    cli.set_e2ee_key(key, _from="community")
    adr = cli.address("community")
    p = cli.query_e2ee_key(adr)
    assert p["key"] == key


def test_encrypt_decrypt(cronos):
    cli = cronos.cosmos_cli()
    key_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    key0 = f"{key_dir}/key0"
    key1 = f"{key_dir}/key1"
    pubkey0 = cli.keygen(o=key0)
    pubkey1 = cli.keygen(o=key1)
    input = f"{key_dir}/input"
    decrypted = f"{key_dir}/input.age"
    data = "Hello, World!"
    with open(input, "w") as file:
        file.write(data)
    cli.encrypt(input, [pubkey0, pubkey1], o=decrypted)
    assert cli.decrypt(decrypted, i=key0) == cli.decrypt(decrypted, i=key1) == data
