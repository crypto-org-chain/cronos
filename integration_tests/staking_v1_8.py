"""
Staking test utilities for v1.8 upgrade testing.

This module contains functions to set up and verify staking state
before and after the v1.8 upgrade.
"""

from dateutil.parser import isoparse

from .cosmoscli import module_address
from .utils import submit_gov_proposal, wait_for_block_time, wait_for_new_blocks


def set_send_enabled(cronos, send_enabled_list):
    authority = module_address("gov")
    msg = "/cosmos.bank.v1beta1.MsgSetSendEnabled"
    submit_gov_proposal(
        cronos,
        msg,
        messages=[
            {
                "@type": msg,
                "authority": authority,
                "send_enabled": send_enabled_list,
            }
        ],
    )


def update_staking_params(cronos, params):
    authority = module_address("gov")
    msg = "/cosmos.staking.v1beta1.MsgUpdateParams"
    submit_gov_proposal(
        cronos,
        msg,
        messages=[
            {
                "@type": msg,
                "authority": authority,
                "params": params,
            }
        ],
    )


def unbond_validators(cli, val_addrs):
    bond_denom = cli.staking_params()["params"].get("bond_denom")
    for val_addr in val_addrs:
        validator_info = cli.validator(val_addr)
        tokens = validator_info["validator"]["tokens"]

        # Unbond all self-delegation to make node0 validator unbonding
        rsp = cli.unbond(val_addr, f"{tokens}{bond_denom}", "validator")
        assert (
            rsp["code"] == 0
        ), f"Failed to unbond validator: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

        # Verify validator status changed to UNBONDING
        validator_info_after = cli.validator(val_addr)
        val_data_after = validator_info_after["validator"]
        status = val_data_after["status"]
        assert status == "BOND_STATUS_UNBONDING", (
            f"Expected validator to be UNBONDING, got {status}. "
            f"Total tokens: {val_data_after.get('tokens')}, "
            f"Min self delegation: {val_data_after.get('min_self_delegation')}"
        )


def unbond_delegations(cli, n, val_addrs):
    bond_denom = cli.staking_params()["params"].get("bond_denom")
    delegator_accounts = []
    stake_amount = 1000000000000000000
    stake_amount_coin = f"{stake_amount}{bond_denom}"

    # fund accounts
    for i in range(n):
        account_name = f"delegator{i}"
        cli.create_account(account_name)
        delegator_addr = cli.address(account_name)
        delegator_accounts.append((account_name, delegator_addr))
        rsp = cli.transfer(
            "community1",
            delegator_addr,
            f"1000000000000000000basetcro,{stake_amount_coin}",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        wait_for_new_blocks(cli, 1)

    for i, (account_name, delegator_addr) in enumerate(delegator_accounts):
        # delegate
        val_addr = val_addrs[i % len(val_addrs)]
        rsp = cli.delegate(val_addr, stake_amount_coin, account_name)
        assert rsp["code"] == 0, f"Failed to delegate: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

        # unbond
        unbond_amount_coin = f"{stake_amount // 2}{bond_denom}"
        rsp = cli.unbond(val_addr, unbond_amount_coin, account_name)
        assert (
            rsp["code"] == 0
        ), f"Failed to unbond delegation {i}: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)
        print(
            f"Unbonding delegation {i} of {unbond_amount_coin}"
            f" from {delegator_addr} to {val_addr}"
        )


def redelegate(cli, n, val_addrs):
    bond_denom = cli.staking_params()["params"].get("bond_denom")
    stake_amount = 1000000000000000000
    stake_amount_coin = f"{stake_amount}{bond_denom}"
    redelegator_accounts = []
    for i in range(n):
        account_name = f"redelegator{i}"
        cli.create_account(account_name)
        redelegator_addr = cli.address(account_name)
        # Alternate between the bonded validators as src/dst
        from_val = val_addrs[i % len(val_addrs)]
        to_val = val_addrs[(i + 1) % len(val_addrs)]

        account_info = {
            "account_name": account_name,
            "delegator_addr": redelegator_addr,
            "validator_src_addr": from_val,
            "validator_dst_addr": to_val,
        }
        redelegator_accounts.append(account_info)

        # Fund the account
        rsp = cli.transfer(
            "community1",
            redelegator_addr,
            f"1000000000000000000basetcro,{stake_amount_coin}",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        wait_for_new_blocks(cli, 1)

    for i, account_info in enumerate(redelegator_accounts):
        account_name = account_info["account_name"]
        redelegator_addr = account_info["delegator_addr"]
        from_val = account_info["validator_src_addr"]
        to_val = account_info["validator_dst_addr"]

        # Delegate to from_val first
        rsp = cli.delegate(from_val, stake_amount_coin, account_name)
        assert rsp["code"] == 0, f"Failed to delegate: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

        # Redelegate
        unbond_amount_coin = f"{stake_amount // 2}{bond_denom}"
        rsp = cli.redelegate(to_val, from_val, unbond_amount_coin, account_name)
        assert rsp["code"] == 0, f"Failed to redelegate {i}: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

        print(f"Redelegation {i} of {unbond_amount_coin} from {from_val} to {to_val}")

    return redelegator_accounts


def get_val_addr_by_status(cli, status=None):
    all_validators = cli.validators()
    unbonding_val_addrs = []
    if status is None:
        return [val["operator_address"] for val in all_validators]
    for val in all_validators:
        if val["status"] == status:
            unbonding_val_addrs.append(val["operator_address"])
    return unbonding_val_addrs


def get_unbonding_delegations(cli):
    all_validators = cli.validators()
    unbonding_delegations = []
    for val in all_validators:
        for ubd in cli.validator_unbonding_delegations(val["operator_address"]):
            if ubd.get("entries"):
                unbonding_delegations.append(
                    {
                        "delegator_addr": ubd["delegator_address"],
                        "validator_addr": ubd["validator_address"],
                    }
                )
    return unbonding_delegations


def get_redelegations(cli, redelegations_before):
    redelegations_after = []
    for redel_before in redelegations_before:
        try:
            redel_responses = cli.redelegations(
                redel_before["delegator_addr"],
                redel_before["validator_src_addr"],
                redel_before["validator_dst_addr"],
            )
            for redel in redel_responses:
                if redel.get("entries"):
                    redelegations_after.append(
                        {
                            "delegator_addr": redel["redelegation"][
                                "delegator_address"
                            ],
                            "validator_src_addr": redel["redelegation"][
                                "validator_src_address"
                            ],
                            "validator_dst_addr": redel["redelegation"][
                                "validator_dst_address"
                            ],
                        }
                    )
        except Exception as e:
            if "NotFound" not in str(e):
                raise
    return redelegations_after


def preupgrade_staking_setup(cli, cronos):
    """
    Set up and verify unbonding validators, unbonding delegations, and
    redelegations before the v1.8 upgrade.

    Creates:
    - 1 unbonding validator (by unbonding all self-delegation from validator 0)
    - 4 unbonding delegations (3 from the 2 remaining bonded validators
    - 3 redelegations (3 between the 2 remaining bonded validators)

    Ensures that the state remains consistent and the unbonding validators,
    unbonding delegations, and redelegations eventually mature after the upgrade.
    """
    # Enable basetcro transfers via governance (disabled in genesis)
    set_send_enabled(cronos, [{"denom": "basetcro", "enabled": True}])

    # Update unbonding_time to 180s
    updated_params = cli.staking_params()["params"].copy()
    updated_params["unbonding_time"] = "180s"
    update_staking_params(cronos, updated_params)
    staking_params = cli.staking_params()["params"]
    new_unbonding_time = staking_params.get("unbonding_time")
    print(f"Unbonding time successfully changed to: {new_unbonding_time}")

    # Identify node0's own validator by querying its self-delegation.
    node0_delegations = cli.validator_delegations(cli.address("validator"))
    node0_val_addr = node0_delegations[0]["delegation"]["validator_address"]

    validators = cli.validators()
    all_val_addrs = [v["operator_address"] for v in validators]
    assert (
        len(all_val_addrs) >= 3
    ), f"Need at least 3 validators, got {len(all_val_addrs)}"
    unbonding_val_addrs = [node0_val_addr]
    # Use the other 2 bonded validators for delegations and redelegations
    delegation_val_addrs = [v for v in all_val_addrs if v != node0_val_addr][:2]

    print("Unbonding 1 validator (via self-delegation unbonding)...")
    unbond_validators(cli, unbonding_val_addrs)

    print("Unbonding 3 delegations...")
    unbond_delegations(cli, 3, delegation_val_addrs)

    print("Redelegating 3 delegations...")
    redelegator_accounts = redelegate(cli, 3, delegation_val_addrs)

    unbonding_validators_before = get_val_addr_by_status(cli, "BOND_STATUS_UNBONDING")
    unbonding_delegations_before = get_unbonding_delegations(cli)
    redelegations_before = get_redelegations(cli, redelegator_accounts)

    print(
        f"Before upgrade: {len(unbonding_validators_before)} unbonding validators, "
        f"{len(unbonding_delegations_before)} unbonding delegations, "
        f"{len(redelegations_before)} redelegations"
    )

    return {
        "unbonding_validators_before": unbonding_validators_before,
        "unbonding_delegations_before": unbonding_delegations_before,
        "redelegations_before": redelegations_before,
    }


def postupgrade_check_staking(cli, preupgrade_staking_info):
    """
    Verify that unbonding validators, unbonding delegations, and redelegations
    persist correctly after the v1.8 upgrade, then wait for all entries to mature
    and assert the final state: 1 unbonded validator, 0 unbonding delegations,
    0 redelegations.
    """
    unbonding_validators_before = preupgrade_staking_info["unbonding_validators_before"]
    unbonding_delegations_before = preupgrade_staking_info[
        "unbonding_delegations_before"
    ]
    redelegations_before = preupgrade_staking_info["redelegations_before"]

    all_validators = cli.validators()

    # --- Immediate post-upgrade checks: entries should still be present ---

    unbonding_validators_after = get_val_addr_by_status(cli, "BOND_STATUS_UNBONDING")
    unbonding_delegations_after = get_unbonding_delegations(cli)
    redelegations_after = get_redelegations(cli, redelegations_before)

    print(
        f"After upgrade: {len(unbonding_validators_after)} unbonding validators, "
        f"{len(unbonding_delegations_after)} unbonding delegations, "
        f"{len(redelegations_after)} redelegations"
    )

    assert len(unbonding_validators_after) == len(
        unbonding_validators_before
    ), "Unbonding validators should still be present after upgrade"
    assert len(unbonding_delegations_after) == len(
        unbonding_delegations_before
    ), "Unbonding delegations should still be present after upgrade"
    assert len(redelegations_after) == len(
        redelegations_before
    ), "Redelegations should still be present after upgrade"

    # --- Collect all completion times and wait for full maturation ---

    completion_times = []
    for val in all_validators:
        for ubd in cli.validator_unbonding_delegations(val["operator_address"]):
            for entry in ubd.get("entries", []):
                completion_times.append(isoparse(entry["completion_time"]))
    # Use stored redelegations to query completion times
    for redel_before in redelegations_before:
        redel_responses = cli.redelegations(
            redel_before["delegator_addr"],
            redel_before["validator_src_addr"],
            redel_before["validator_dst_addr"],
        )
        for redel in redel_responses:
            for entry in redel.get("entries", []):
                completion_times.append(
                    isoparse(entry["redelegation_entry"]["completion_time"])
                )

    assert completion_times, "No completion times found; nothing to wait for"
    max_completion_time = max(completion_times)
    print(f"Waiting for all entries to mature by {max_completion_time}...")
    wait_for_block_time(cli, max_completion_time)

    # A couple extra blocks for the chain to process the maturations
    wait_for_new_blocks(cli, 2)

    # --- Post-maturation assertions ---

    unbonded_or_unbonding_vals = get_val_addr_by_status(
        cli, "BOND_STATUS_UNBONDED"
    ) + get_val_addr_by_status(cli, "BOND_STATUS_UNBONDING")
    assert len(unbonded_or_unbonding_vals) == 0, (
        f"Expected 0 unbonded/unbonding validators after maturation, "
        f"got {len(unbonded_or_unbonding_vals)}"
    )

    remaining_ubds = get_unbonding_delegations(cli)
    assert (
        len(remaining_ubds) == 0
    ), f"Expected no unbonding delegations after maturation, got {len(remaining_ubds)}"

    remaining_redels = get_redelegations(cli, redelegations_before)
    assert (
        len(remaining_redels) == 0
    ), f"Expected no redelegations after maturation, got {len(remaining_redels)}"

    print(
        "Post-upgrade check passed: 1 unbonded validator removed from the "
        "validator set, all unbonding delegations and redelegations matured"
    )
