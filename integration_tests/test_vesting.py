import time


def test_create_account(cronos):
    """
    test create vesting account is disabled:
    """
    cli = cronos.cosmos_cli()
    src = "vesting"
    addr = cli.create_account(src)["address"]
    denom = "basetcro"
    balance = cli.balance(addr, denom)
    assert balance == 0
    amount = 10000
    amt = f"{amount}{denom}"
    end_time = int(time.time()) + 3000
    res = cli.create_vesting_account(addr, amt, end_time, from_="validator")
    assert res["code"] != 0
    assert "vesting messages are not supported" in res["raw_log"]
