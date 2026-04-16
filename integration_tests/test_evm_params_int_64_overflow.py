"""
Integration tests for EVM module int64 overflow parameter validation.

This test suite validates that the EVM module properly handles different values
of the header_hash_num and history_serve_window parameters, including edge cases
that could cause int64 overflow.

Test scenarios (both parameters tested together):
1. value = 0 (valid, should succeed - no minimum check in ValidateInt64Overflow)
2. value = 100 (valid, should succeed)
3. value = MaxInt64 (valid, should succeed)
4. value = MaxInt64 + 1 (overflow case, should be rejected)
5. value = 2^64-1 (UINT64_MAX, overflow case, should be rejected)
"""

import pytest

from .cosmoscli import module_address
from .utils import submit_gov_proposal, wait_for_new_blocks

pytestmark = pytest.mark.evm

MAX_INT64 = (1 << 63) - 1  # 9223372036854775807
UINT64_MAX = (1 << 64) - 1  # 18446744073709551615


def get_evm_params(cli):
    """Query current EVM module parameters."""
    params = cli.query_params("evm")
    return params


def submit_evm_param_update(cronos, params):
    """
    Submit a governance proposal to update EVM module parameters.

    Args:
        cronos: Cronos cluster instance
        params: Complete params dict with updated values

    Returns:
        True if proposal passes, False if it fails
    """
    authority = module_address("gov")
    msg = "/ethermint.evm.v1.MsgUpdateParams"

    try:
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
        return True
    except (AssertionError, Exception) as e:
        print(f"Proposal failed as expected: {e}")
        return False


def prepare_evm_params(cli, header_hash_num, history_serve_window):
    """
    Prepare EVM params for update by querying current params and applying updates.

    Args:
        cli: Cosmos CLI instance
        header_hash_num: Value to set for header_hash_num
        history_serve_window: Value to set for history_serve_window

    Returns:
        Updated params dict
    """
    params = get_evm_params(cli)
    params["header_hash_num"] = str(header_hash_num)
    params["history_serve_window"] = str(history_serve_window)
    return params


def test_evm_params_zero(cronos):
    """
    Test that header_hash_num = 0 and history_serve_window = 0 are valid.

    Zero is a valid value as ValidateInt64Overflow only checks for values
    exceeding MaxInt64.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_evm_params(cli)
    initial_header_hash_num = initial_params["header_hash_num"]
    initial_history_serve_window = initial_params["history_serve_window"]
    print(f"Initial header_hash_num: {initial_header_hash_num}")
    print(f"Initial history_serve_window: {initial_history_serve_window}")

    test_value = 0
    print(f"Attempting to set both params to {test_value}")

    params = prepare_evm_params(cli, test_value, test_value)
    success = submit_evm_param_update(cronos, params)

    # Should succeed (0 is valid)
    assert success, f"Proposal should succeed for value = {test_value}"

    wait_for_new_blocks(cli, 2)
    updated_params = get_evm_params(cli)
    print(f"Updated header_hash_num: {updated_params['header_hash_num']}")
    print(f"Updated history_serve_window: {updated_params['history_serve_window']}")

    assert updated_params["header_hash_num"] == str(
        test_value
    ), f"header_hash_num should be {test_value}"
    assert updated_params["history_serve_window"] == str(
        test_value
    ), f"history_serve_window should be {test_value}"

    # Verify chain continues to produce blocks
    wait_for_new_blocks(cli, 3)
    print(f"Chain continues to produce blocks with both params = {test_value}")


def test_evm_params_valid(cronos):
    """
    Test that valid values (100) work correctly for both parameters.

    This verifies that the parameter update mechanism works for valid values.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_evm_params(cli)
    print(f"Initial header_hash_num: {initial_params['header_hash_num']}")
    print(f"Initial history_serve_window: {initial_params['history_serve_window']}")

    test_value = 100
    print(f"Attempting to set both params to {test_value}")

    params = prepare_evm_params(cli, test_value, test_value)
    success = submit_evm_param_update(cronos, params)

    assert success, f"Valid proposal should succeed with value {test_value}"

    wait_for_new_blocks(cli, 2)
    updated_params = get_evm_params(cli)
    print(f"Updated header_hash_num: {updated_params['header_hash_num']}")
    print(f"Updated history_serve_window: {updated_params['history_serve_window']}")

    assert updated_params["header_hash_num"] == str(
        test_value
    ), f"header_hash_num should be {test_value}"
    assert updated_params["history_serve_window"] == str(
        test_value
    ), f"history_serve_window should be {test_value}"

    # Verify chain continues to produce blocks
    wait_for_new_blocks(cli, 3)
    print(f"Chain continues to produce blocks with both params = {test_value}")


def test_evm_params_max_int64_boundary(cronos):
    """
    Test the boundary case: both params = MaxInt64.

    This value should be valid as it's the maximum positive value
    that fits in an int64.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_evm_params(cli)
    print(f"Initial header_hash_num: {initial_params['header_hash_num']}")
    print(f"Initial history_serve_window: {initial_params['history_serve_window']}")

    test_value = MAX_INT64
    print(f"Attempting to set both params to {test_value} (MaxInt64)")

    params = prepare_evm_params(cli, test_value, test_value)
    success = submit_evm_param_update(cronos, params)

    assert success, f"Proposal should succeed with value = {test_value}"

    wait_for_new_blocks(cli, 2)
    updated_params = get_evm_params(cli)
    print(f"Updated header_hash_num: {updated_params['header_hash_num']}")
    print(f"Updated history_serve_window: {updated_params['history_serve_window']}")

    assert updated_params["header_hash_num"] == str(
        test_value
    ), f"header_hash_num should be {test_value}"
    assert updated_params["history_serve_window"] == str(
        test_value
    ), f"history_serve_window should be {test_value}"

    wait_for_new_blocks(cli, 3)
    print(f"Chain continues to produce blocks with both params = {test_value}")


def test_evm_params_just_over_max_int64(cronos):
    """
    Test the edge case: both params = MaxInt64 + 1.

    This is the smallest value that would overflow int64 and should be rejected.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_evm_params(cli)
    initial_header_hash_num = initial_params["header_hash_num"]
    initial_history_serve_window = initial_params["history_serve_window"]
    print(f"Initial header_hash_num: {initial_header_hash_num}")
    print(f"Initial history_serve_window: {initial_history_serve_window}")

    overflow_value = MAX_INT64 + 1
    print(f"Attempting to set both params to {overflow_value}")

    params = prepare_evm_params(cli, overflow_value, overflow_value)
    success = submit_evm_param_update(cronos, params)

    assert not success, f"Proposal should fail for value = {overflow_value}"

    wait_for_new_blocks(cli, 1)
    current_params = get_evm_params(cli)
    assert (
        current_params["header_hash_num"] == initial_header_hash_num
    ), "header_hash_num should not change"
    assert (
        current_params["history_serve_window"] == initial_history_serve_window
    ), "history_serve_window should not change"


def test_evm_params_uint64_max(cronos):
    """
    Test that both params = 2^64-1 (UINT64_MAX) is properly rejected.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_evm_params(cli)
    initial_header_hash_num = initial_params["header_hash_num"]
    initial_history_serve_window = initial_params["history_serve_window"]
    print(f"Initial header_hash_num: {initial_header_hash_num}")
    print(f"Initial history_serve_window: {initial_history_serve_window}")

    overflow_value = UINT64_MAX
    print(f"Attempting to set both params to {overflow_value} (2^64 - 1)")

    params = prepare_evm_params(cli, overflow_value, overflow_value)
    success = submit_evm_param_update(cronos, params)

    assert not success, f"Proposal should fail for value = {overflow_value}"

    wait_for_new_blocks(cli, 1)
    current_params = get_evm_params(cli)
    assert (
        current_params["header_hash_num"] == initial_header_hash_num
    ), "header_hash_num should not change"
    assert (
        current_params["history_serve_window"] == initial_history_serve_window
    ), "history_serve_window should not change"
