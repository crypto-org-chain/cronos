"""
Integration tests for mint module BlocksPerYear parameter validation.

This test suite validates that the mint module properly handles different values
of the blocks_per_year parameter, including edge cases that could cause overflow
or panic conditions.

Test scenarios:
1. blocks_per_year = 0 (invalid, should be rejected)
2. blocks_per_year = 100 (valid, should succeed)
3. blocks_per_year = MaxInt64 (valid, should succeed)
4. blocks_per_year = MaxInt64 + 1 (overflow case, should be rejected)
5. blocks_per_year = 2^64-1 (UINT64_MAX, overflow case, should be rejected)
"""

import pytest

from .cosmoscli import module_address
from .utils import submit_gov_proposal, wait_for_new_blocks

pytestmark = pytest.mark.mint

MAX_INT64 = (1 << 63) - 1  # 9223372036854775807
UINT64_MAX = (1 << 64) - 1  # 18446744073709551615


def normalize_legacy_dec(value: str) -> str:
    """
    Ensure math.LegacyDec strings have an explicit decimal point (scale 18).
    This matches the Cosmos SDK's LegacyDec format.
    """
    if not value:
        return "0.000000000000000000"
    if "." in value:
        return value
    sign = ""
    if value[0] == "-":
        sign = "-"
        value = value[1:]
    stripped = value.lstrip("0")
    if not stripped:
        return "0.000000000000000000"
    padded = stripped.rjust(19, "0")
    int_part = padded[:-18] or "0"
    frac_part = padded[-18:]
    return f"{sign}{int_part}.{frac_part}"


def get_mint_params(cli):
    """Query current mint module parameters."""
    params = cli.query_params("mint")
    return params


def submit_mint_param_update(cronos, blocks_per_year_value):
    """
    Submit a governance proposal to update mint module parameters.

    Args:
        cronos: Cronos cluster instance
        blocks_per_year_value: Value to set for blocks_per_year parameter

    Returns:
        True if proposal passes, False if it fails
    """
    cli = cronos.cosmos_cli()

    params = get_mint_params(cli)

    # Normalize legacy decimal fields
    for dec_key in (
        "inflation_rate_change",
        "inflation_max",
        "inflation_min",
        "goal_bonded",
    ):
        if dec_key in params and isinstance(params[dec_key], str):
            params[dec_key] = normalize_legacy_dec(params[dec_key])

    # Update blocks_per_year to the test value
    params["blocks_per_year"] = str(blocks_per_year_value)

    authority = module_address("gov")
    msg = "/cosmos.mint.v1beta1.MsgUpdateParams"

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


def test_mint_blocks_per_year_zero(cronos):
    """
    Test that blocks_per_year = 0 is properly rejected.

    Zero is an invalid value because it would cause division by zero
    in the minting calculations.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_mint_params(cli)
    initial_blocks_per_year = initial_params["blocks_per_year"]
    print(f"Initial blocks_per_year: {initial_blocks_per_year}")
    test_value = 0

    # Attempt to update to 0 (should fail)
    print(f"Attempting to set blocks_per_year to {test_value}")
    success = submit_mint_param_update(cronos, test_value)

    assert not success, f"Proposal should fail for blocks_per_year = {test_value}"
    wait_for_new_blocks(cli, 1)
    current_params = get_mint_params(cli)
    assert (
        current_params["blocks_per_year"] == initial_blocks_per_year
    ), "Blocks per year should not change"


def test_mint_blocks_per_year_valid(cronos):
    """
    Test that a valid blocks_per_year value (100) works correctly.

    This verifies that the parameter update mechanism works for valid values.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_mint_params(cli)
    initial_blocks_per_year = initial_params["blocks_per_year"]
    print(f"Initial blocks_per_year: {initial_blocks_per_year}")

    # Update to a valid value (100)
    test_value = 100
    print(f"Attempting to set blocks_per_year to {test_value}")
    success = submit_mint_param_update(cronos, test_value)

    # Should succeed
    assert (
        success
    ), f"Valid blocks_per_year proposal should succeed with value {test_value}"

    # Wait for proposal execution
    wait_for_new_blocks(cli, 2)

    # Verify the chain is still producing blocks (no panic)
    updated_params = get_mint_params(cli)
    updated_blocks_per_year = updated_params["blocks_per_year"]
    print(f"Updated blocks_per_year: {updated_blocks_per_year}")

    # Verify the value was updated correctly
    assert updated_blocks_per_year == str(
        test_value
    ), f"blocks_per_year should be {test_value}, got {updated_blocks_per_year}"

    # Verify chain continues to produce blocks after the update
    # This ensures the BeginBlocker doesn't panic with the new value
    wait_for_new_blocks(cli, 3)
    print(f"Chain continues to produce blocks with valid blocks_per_year {test_value}")


def test_mint_blocks_per_year_max_int64_boundary(cronos):
    """
    Test the boundary case: blocks_per_year = MaxInt64.

    This value should be valid as it's the maximum positive value
    that fits in an int64.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_mint_params(cli)
    initial_blocks_per_year = initial_params["blocks_per_year"]
    print(f"Initial blocks_per_year: {initial_blocks_per_year}")

    # Update to MaxInt64 (should be valid)
    test_value = MAX_INT64
    print(f"Attempting to set blocks_per_year to {test_value} (MaxInt64)")

    success = submit_mint_param_update(cronos, test_value)
    assert success, f"Proposal should succeed with blocks_per_year = {test_value}"

    wait_for_new_blocks(cli, 2)
    updated_params = get_mint_params(cli)
    updated_blocks_per_year = updated_params["blocks_per_year"]
    print(f"Updated blocks_per_year: {updated_blocks_per_year}")
    assert updated_blocks_per_year == str(
        test_value
    ), f"blocks_per_year should be {test_value}, got {updated_blocks_per_year}"

    wait_for_new_blocks(cli, 3)
    print(f"Chain continues to produce blocks with blocks_per_year = {test_value}")


def test_mint_blocks_per_year_just_over_max_int64(cronos):
    """
    Test the edge case: blocks_per_year = MaxInt64 + 1.

    This is the smallest value that would overflow int64 and should be rejected.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_mint_params(cli)
    initial_blocks_per_year = initial_params["blocks_per_year"]
    print(f"Initial blocks_per_year: {initial_blocks_per_year}")

    # Attempt to update to MaxInt64 + 1 (should fail or be rejected)
    overflow_value = MAX_INT64 + 1
    print(f"Attempting to set blocks_per_year to {overflow_value}")

    success = submit_mint_param_update(cronos, overflow_value)

    assert not success, f"Proposal should fail for blocks_per_year = {overflow_value}"
    wait_for_new_blocks(cli, 1)
    current_params = get_mint_params(cli)
    assert (
        current_params["blocks_per_year"] == initial_blocks_per_year
    ), "Blocks per year should not change"


def test_mint_blocks_per_year_uint64_max(cronos):
    """
    Test that blocks_per_year = 2^64-1 (UINT64_MAX) is properly rejected.
    """
    cli = cronos.cosmos_cli()

    # Get initial params
    initial_params = get_mint_params(cli)
    initial_blocks_per_year = initial_params["blocks_per_year"]
    print(f"Initial blocks_per_year: {initial_blocks_per_year}")

    # Attempt to update to UINT64_MAX (2^64 - 1)
    # This value fits in uint64 but severely overflows int64
    overflow_value = UINT64_MAX
    print(f"Attempting to set blocks_per_year to {overflow_value} (2^64 - 1)")

    success = submit_mint_param_update(cronos, overflow_value)

    assert not success, f"Proposal should fail for blocks_per_year = {overflow_value}"
    wait_for_new_blocks(cli, 1)
    current_params = get_mint_params(cli)
    assert (
        current_params["blocks_per_year"] == initial_blocks_per_year
    ), "Blocks per year should not change"
