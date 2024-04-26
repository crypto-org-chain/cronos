from .utils import prepare_cipherfile


def test_encrypt_decrypt(cronos):
    cli = cronos.cosmos_cli()
    cli1 = cronos.cosmos_cli(1)
    name0 = "key0"
    name1 = "key1"
    content = "Hello World!"
    cipherfile = prepare_cipherfile(cli, cli1, name0, name1, content)
    assert cli.decrypt(cipherfile, identity=name0) == content
    assert cli1.decrypt(cipherfile, identity=name1) == content
