import base64


def test_set_key(cronos):
    cli = cronos.cosmos_cli()
    key = base64.b64encode(b"new_key").decode("utf-8")
    cli.set_e2ee_key(key, _from="community")
    adr = cli.address("community")
    p = cli.query_e2ee_key(adr)
    assert p["key"] == key
