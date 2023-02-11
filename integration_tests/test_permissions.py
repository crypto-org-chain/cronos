from .utils import ADDRS, eth_to_bech32, wait_for_new_blocks


def test_permissions_updates(cronos):
    acc = eth_to_bech32(ADDRS["signer1"])
    cli = cronos.cosmos_cli()
    rsp = cli.query_permissions(acc)
    print("permissions", rsp)
    assert rsp["can_change_token_mapping"] is False
    assert rsp["can_turn_bridge"] is False

    # update permissions
    rsp = cli.update_permissions(acc, 3, from_="community")
    assert rsp["code"] != 0, "should not have the permission"

    rsp = cli.update_permissions(acc, 3, from_="validator")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)

    rsp = cli.query_permissions(acc)
    print("permissions", rsp)
    assert rsp["can_change_token_mapping"] is True
    assert rsp["can_turn_bridge"] is True
