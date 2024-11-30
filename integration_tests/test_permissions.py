import os

from .utils import ADDRS, eth_to_bech32, wait_for_new_blocks


def test_permissions_updates(cronos):
    """
    - test permissions updates
    - reproduce an iavl prune issue: https://github.com/cosmos/iavl/pull/1007
    """
    acc = eth_to_bech32(ADDRS["signer1"])
    cli = cronos.cosmos_cli(1)  # node1 is iavl
    cli.create_account("community", os.environ["COMMUNITY_MNEMONIC"])
    cli.create_account("admin", os.environ["VALIDATOR1_MNEMONIC"])
    rsp = cli.query_permissions(acc)
    print("permissions", rsp)
    assert rsp["can_change_token_mapping"] is False
    assert rsp["can_turn_bridge"] is False

    # update permissions
    rsp = cli.update_permissions(acc, 3, from_="community")
    assert rsp["code"] != 0, "should not have the permission"

    rsp = cli.update_permissions(acc, 3, from_="admin")
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cli, 5)

    rsp = cli.query_permissions(acc)
    print("permissions", rsp)
    assert rsp["can_change_token_mapping"] is True
    assert rsp["can_turn_bridge"] is True

    cronos.supervisorctl("stop", "cronos_777-1-node1")
    print(cli.prune())
    cronos.supervisorctl("start", "cronos_777-1-node1")

    rsp = cli.update_permissions(acc, 4, from_="admin")
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cli, 5)
