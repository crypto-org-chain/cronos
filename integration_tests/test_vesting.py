def test_create_account(cronos):
    """
    test create vesting account tx works:
    """
    cli = cronos.cosmos_cli()
    src = "vesting"
    addr = cli.create_account(src)["address"]
    denom = "basetcro"
    balance = cli.balance(addr, denom)
    assert balance == 0
    amount = 10000
    fee = 4000000000000000
    amt = f"{amount}{denom}"
    res = cli.create_vesting_account(addr, amt, from_="validator", fees=f"{fee}{denom}")
    assert res["code"] == 0, res["raw_log"]
    balance = cli.balance(addr, denom)
    assert balance == amount
