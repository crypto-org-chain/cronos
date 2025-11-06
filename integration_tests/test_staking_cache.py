"""
Test staking cache functionality with different cache sizes.

This test verifies that staking operations work correctly across nodes
with different cache size configurations:
- Node 0: cache-size = -1 (disabled)
- Node 1: cache-size = 0 (unlimited)
- Node 2: cache-size = 1
- Node 3: cache-size = 2
- Node 4: cache-size = 3
- Node 5: cache-size = 100
- Node 6: cache-size = 1000

Test scenarios:
1. Multiple unbonding operations from different delegators
2. Multiple redelegations between validators
3. Unbonding validators by removing all self-delegation
"""

import pytest

from .network import setup_custom_cronos
from .utils import wait_for_new_blocks

pytestmark = pytest.mark.staking


@pytest.fixture(scope="function")
def cronos_staking_cache(tmp_path_factory):
    """Setup cronos cluster with different staking cache sizes per node."""
    from pathlib import Path

    path = tmp_path_factory.mktemp("staking_cache")
    yield from setup_custom_cronos(
        path,
        26650,
        Path(__file__).parent / "configs/staking_cache.jsonnet",
    )


def get_delegator_address(cli, account_name):
    """Get delegator address for a specific account."""
    return cli.address(account_name)


def test_staking_cache_multiple_unbonding(cronos_staking_cache):
    """
    Test multiple unbonding operations across nodes with different cache sizes.

    This test performs multiple unbonding operations from different accounts
    to different validators and verifies that all nodes maintain consistent state
    regardless of their cache configuration.
    """
    cronos = cronos_staking_cache

    # Get CLI instances for different nodes
    cli = cronos.cosmos_cli()

    # Get validator addresses
    validators = []
    for i in range(7):
        val_addr = cronos.cosmos_cli(i).address("validator", bech="val")
        validators.append(val_addr)

    print(f"Validators: {validators}")

    # Delegate some tokens from rich account to all validators
    rich_addr = get_delegator_address(cli, "rich")
    delegation_amount = "1000000000000000000stake"  # 1 stake token

    print("\n=== Phase 1: Delegating to validators ===")
    for i, val_addr in enumerate(validators):
        print(f"Delegating to validator {i}: {val_addr}")
        rsp = cli.delegate_amount(
            val_addr,
            delegation_amount,
            "rich",
        )
        assert rsp["code"] == 0, f"Delegation failed: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

    # Verify delegations were successful on all nodes
    print("\n=== Verifying delegations on all nodes ===")
    delegation_counts = []
    for node_idx in range(7):
        node_cli = cronos.cosmos_cli(node_idx)
        delegations = node_cli.get_delegated_amount(rich_addr)
        delegation_responses = (
            delegations.get("delegation_responses", []) if delegations else []
        )
        count = len(delegation_responses)
        delegation_counts.append(count)
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]
        print(f"Node {node_idx} (cache-size={cache_size}): " f"{count} delegations")

    # Verify all nodes have the same number of delegations
    assert (
        len(set(delegation_counts)) == 1
    ), f"Nodes have different delegation counts: {delegation_counts}"
    print(f"✓ All nodes consistent: {delegation_counts[0]} delegations each")

    # Perform multiple unbonding operations from different accounts
    print("\n=== Phase 2: Multiple unbonding operations ===")

    # Unbond from rich account
    unbond_amount = "500000000000000000stake"  # 0.5 stake token
    unbonding_ops = []

    for i in range(3):  # Unbond from first 3 validators
        val_addr = validators[i]
        print(f"Unbonding from validator {i}: {val_addr}")
        rsp = cli.unbond_amount(val_addr, unbond_amount, "rich")
        assert rsp["code"] == 0, f"Unbonding failed: {rsp.get('raw_log', rsp)}"
        unbonding_ops.append((rich_addr, val_addr, unbond_amount))
        wait_for_new_blocks(cli, 2)

    # Delegate from alice and then unbond
    alice_addr = get_delegator_address(cli, "alice")
    print("\nDelegating from alice to validator 3")
    rsp = cli.delegate_amount(validators[3], delegation_amount, "alice")
    assert rsp["code"] == 0, f"Alice delegation failed: {rsp.get('raw_log', rsp)}"

    wait_for_new_blocks(cli, 2)

    print("Unbonding from alice")
    rsp = cli.unbond_amount(validators[3], unbond_amount, "alice")
    assert rsp["code"] == 0, f"Alice unbonding failed: {rsp.get('raw_log', rsp)}"
    unbonding_ops.append((alice_addr, validators[3], unbond_amount))

    # Delegate from bob and then unbond
    bob_addr = get_delegator_address(cli, "bob")
    print("\nDelegating from bob to validator 4")
    rsp = cli.delegate_amount(validators[4], delegation_amount, "bob")
    assert rsp["code"] == 0, f"Bob delegation failed: {rsp.get('raw_log', rsp)}"

    wait_for_new_blocks(cli, 2)

    print("Unbonding from bob")
    rsp = cli.unbond_amount(validators[4], unbond_amount, "bob")
    assert rsp["code"] == 0, f"Bob unbonding failed: {rsp.get('raw_log', rsp)}"
    unbonding_ops.append((bob_addr, validators[4], unbond_amount))

    wait_for_new_blocks(cli, 2)

    # Verify unbonding entries exist on all nodes
    print("\n=== Phase 3: Verifying unbonding entries on all nodes ===")

    # Get unique delegator addresses from unbonding operations
    unique_delegators = set(delegator_addr for delegator_addr, _, _ in unbonding_ops)

    # Collect total unbonding delegation counts from each node
    total_unbonding_counts = []

    for node_idx in range(7):
        node_cli = cronos.cosmos_cli(node_idx)
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

        # Get unbonding delegations for all delegators
        total_count = 0
        for delegator_addr in unique_delegators:
            unbonding = node_cli.get_unbonding_delegations(delegator_addr)
            count = len(unbonding) if unbonding else 0
            total_count += count

        total_unbonding_counts.append(total_count)
        msg = (
            f"Node {node_idx} (cache-size={cache_size}): "
            f"{total_count} total unbonding delegations"
        )
        print(msg)

    # Verify all nodes have the same total count
    assert (
        len(set(total_unbonding_counts)) == 1
    ), f"Nodes have different unbonding delegation counts: {total_unbonding_counts}"
    msg = (
        f"Node {node_idx} (cache-size={cache_size}): "
        f"{total_count} total unbonding delegations"
    )
    print(msg)

    # Wait for unbonding period to complete (20 seconds, 20 blocks)
    print("\n=== Phase 4: Waiting for unbonding period to complete ===")
    print("Waiting 60 seconds for unbonding delegations to mature...")
    wait_for_new_blocks(cli, 60)

    # Verify unbonding delegations are now empty/matured on all nodes
    print(
        "\n=== Phase 5: Verifying unbonding delegations matured "
        "(should be empty) ==="
    )
    matured_unbonding_counts = []

    for node_idx in range(7):
        node_cli = cronos.cosmos_cli(node_idx)
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

        # Get unbonding delegations for all delegators
        total_count = 0
        for delegator_addr in unique_delegators:
            unbonding = node_cli.get_unbonding_delegations(delegator_addr)
            count = len(unbonding) if unbonding else 0
            total_count += count

        matured_unbonding_counts.append(total_count)
        msg = (
            f"Node {node_idx} (cache-size={cache_size}): "
            f"{total_count} unbonding delegations remaining"
        )
        print(msg)

    # Verify all nodes agree that unbonding delegations are empty
    assert (
        len(set(matured_unbonding_counts)) == 1
    ), f"Nodes have different matured unbonding counts: {matured_unbonding_counts}"
    assert matured_unbonding_counts[0] == 0, (
        f"Expected 0 unbonding delegations after maturation "
        f"but got {matured_unbonding_counts[0]}"
    )
    msg = (
        f"✓ All nodes consistent: {matured_unbonding_counts[0]} "
        f"unbonding delegations (all matured)"
    )
    print(msg)

    print("\n=== Test completed successfully ===")


def test_staking_cache_multiple_redelegations(cronos_staking_cache):
    """
    Test multiple redelegation operations across nodes with different cache sizes.

    This test performs multiple redelegations between validators and verifies
    that all nodes maintain consistent state regardless of cache configuration.
    """
    cronos = cronos_staking_cache
    cli = cronos.cosmos_cli()

    # Get validator addresses
    validators = []
    for i in range(7):
        val_addr = cronos.cosmos_cli(i).address("validator", bech="val")
        validators.append(val_addr)

    print(f"Validators: {validators}")

    # Get delegator addresses
    charlie_addr = get_delegator_address(cli, "charlie")
    alice_addr = get_delegator_address(cli, "alice")

    delegation_amount = "2000000000000000000stake"  # 2 stake tokens

    print("\n=== Phase 1: Initial delegations ===")

    # Charlie delegates to first 3 validators
    print("Charlie delegating to validators 0, 1, 2")
    for i in range(3):
        print(f"  Delegating to validator {i}")
        rsp = cli.delegate_amount(validators[i], delegation_amount, "charlie")
        assert rsp["code"] == 0, f"Charlie delegation failed: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

    # Alice delegates to validators 1, 2, 3
    print("Alice delegating to validators 1, 2, 3")
    for i in range(1, 4):
        print(f"  Delegating to validator {i}")
        rsp = cli.delegate_amount(validators[i], delegation_amount, "alice")
        assert rsp["code"] == 0, f"Alice delegation failed: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

    # Perform multiple redelegations
    print("\n=== Phase 2: Redelegations ===")
    redelegate_amount = "1000000000000000000stake"  # 1 stake token

    # Charlie: Redelegate once (from validator 0 to validator 3)
    print("Charlie redelegating from validator 0 to validator 3")
    rsp = cli.redelegate_amount(
        validators[3], validators[0], redelegate_amount, "charlie"  # to  # from
    )
    assert rsp["code"] == 0, f"Charlie redelegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Alice: First redelegation (from validator 1 to validator 0)
    print("Alice redelegating from validator 1 to validator 0")
    rsp = cli.redelegate_amount(
        validators[0], validators[1], redelegate_amount, "alice"  # to  # from
    )
    assert rsp["code"] == 0, f"Alice redelegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Alice: Second redelegation (from validator 2 to validator 4)
    print("Alice redelegating from validator 2 to validator 4")
    rsp = cli.redelegate_amount(
        validators[4], validators[2], redelegate_amount, "alice"  # to  # from
    )
    assert rsp["code"] == 0, f"Alice redelegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Verify redelegation consistency across all nodes
    print("\n=== Phase 3: Verifying redelegation consistency on all nodes ===")

    # Expected redelegations: (src_validator, dst_validator, delegator_name)
    expected_redelegations = [
        # Charlie: validator 0 -> 3
        (validators[0], validators[3], charlie_addr, "Charlie"),
        # Alice: validator 1 -> 0
        (validators[1], validators[0], alice_addr, "Alice"),
        # Alice: validator 2 -> 4
        (validators[2], validators[4], alice_addr, "Alice"),
    ]

    for src_val, dst_val, delegator_addr, delegator_name in expected_redelegations:
        redelegation_counts = []

        msg = (
            f"\nChecking {delegator_name}'s redelegation from "
            f"{src_val}... to {dst_val}...:"
        )
        print(msg)
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

            redelegations = node_cli.get_redelegations(delegator_addr, src_val, dst_val)
            count = len(redelegations) if redelegations else 0
            redelegation_counts.append(count)
            msg = (
                f"  Node {node_idx} (cache-size={cache_size}): "
                f"{count} redelegation(s)"
            )
            print(msg)

        # Verify consistency across nodes
        assert len(set(redelegation_counts)) == 1, (
            f"{delegator_name}'s redelegation has different counts "
            f"across nodes: {redelegation_counts}"
        )

        # Verify we have exactly 1 redelegation entry
        assert redelegation_counts[0] == 1, (
            f"{delegator_name}'s redelegation expected 1 entry "
            f"but got {redelegation_counts[0]}"
        )

        print(f"  ✓ All nodes consistent: {redelegation_counts[0]} redelegation entry")

    # Wait for redelegation completion period (20 seconds, 20 blocks)
    print("\n=== Phase 4: Waiting for redelegation completion period ===")
    print("Waiting 60 seconds for redelegations to complete...")
    wait_for_new_blocks(cli, 60)

    # Verify redelegations are now empty/completed on all nodes
    print("\n=== Phase 5: Verifying redelegations completed (should be empty) ===")

    for src_val, dst_val, delegator_addr, delegator_name in expected_redelegations:
        matured_redelegation_counts = []

        msg = (
            f"\nChecking {delegator_name}'s redelegation from "
            f"{src_val}... to {dst_val}... (after completion):"
        )
        print(msg)
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

            try:
                redelegations = node_cli.get_redelegations(
                    delegator_addr, src_val, dst_val
                )
                count = len(redelegations) if redelegations else 0
                matured_redelegation_counts.append(count)
                msg = (
                    f"  Node {node_idx} (cache-size={cache_size}): "
                    f"{count} redelegation(s) remaining"
                )
                print(msg)
            except Exception as e:
                # "not found" errors are expected when redelegations have matured
                error_str = str(e).lower()
                if "not found" in error_str:
                    matured_redelegation_counts.append(0)
                    msg = (
                        f"  Node {node_idx} (cache-size={cache_size}): "
                        f"0 redelegation(s) remaining"
                    )
                    print(msg)
                else:
                    # Unexpected error - fail the test
                    raise

        # Verify consistency across nodes
        assert len(set(matured_redelegation_counts)) == 1, (
            f"{delegator_name}'s matured redelegation has different "
            f"counts across nodes: {matured_redelegation_counts}"
        )

        # Verify redelegation is now complete (count should be 0)
        assert matured_redelegation_counts[0] == 0, (
            f"{delegator_name}'s redelegation expected 0 entries "
            f"after completion but got {matured_redelegation_counts[0]}"
        )

        msg = (
            f"  ✓ All nodes consistent: {matured_redelegation_counts[0]} "
            f"redelegation entries (completed)"
        )
        print(msg)

    print("\n=== Test completed successfully ===")


def test_staking_cache_unbonding_validator(cronos_staking_cache):
    """
    Test unbonding validators by removing all self-delegation.

    This test verifies that when validators unbond all their self-delegation,
    they transition to unbonding state correctly across all nodes
    with different cache configurations.

    Tests 3 validators: nodes 4, 5, and 6 (cache-size=3, 100, 1000)
    """
    cronos = cronos_staking_cache

    # We'll unbond validators from nodes 4, 5, and 6
    test_node_indices = [4, 5, 6]
    test_validators = []

    print("\n=== Testing validator unbonding (3 validators) ===")

    # Collect validator info for all test nodes
    for test_node_idx in test_node_indices:
        cli = cronos.cosmos_cli(test_node_idx)
        val_addr = cli.address("validator", bech="val")
        val_account = cli.address("validator")
        test_validators.append(
            {
                "node_idx": test_node_idx,
                "cli": cli,
                "val_addr": val_addr,
                "val_account": val_account,
            }
        )
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][test_node_idx]
        print(
            f"Node {test_node_idx} (cache-size={cache_size}): "
            f"Validator address: {val_addr}"
        )

    # Get initial validator status on all nodes
    print("\n=== Phase 1: Initial validator status ===")
    for node_idx in range(7):
        node_cli = cronos.cosmos_cli(node_idx)
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]
        validators = node_cli.validators()

        print(
            f"Node {node_idx} (cache-size={cache_size}): {len(validators)} validators"
        )

        # Verify all test validators are present
        for test_val_info in test_validators:
            test_val = None
            for v in validators:
                if v["operator_address"] == test_val_info["val_addr"]:
                    test_val = v
                    break

            assert test_val is not None, (
                f"Node {node_idx} (cache-size={cache_size}): "
                f"Validator {test_val_info['val_addr']} not found in "
                f"initial status check"
            )

    # Query each validator's actual total tokens and set min_self_delegation
    print(
        "\n=== Phase 2: Query validators' actual tokens and set min_self_delegation ==="
    )

    for test_val_info in test_validators:
        cli = test_val_info["cli"]
        val_addr = test_val_info["val_addr"]
        node_idx = test_val_info["node_idx"]
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

        print(
            f"\nNode {node_idx} (cache-size={cache_size}): "
            f"Processing validator {val_addr}"
        )

        validator_info = cli.validator(val_addr)
        assert (
            validator_info and "validator" in validator_info
        ), f"Failed to query validator {val_addr}"

        actual_tokens = int(validator_info["validator"].get("tokens", "0"))
        print(f"  Validator's actual total tokens: {actual_tokens}")
        test_val_info["actual_tokens"] = actual_tokens

        # Set min_self_delegation to current tokens to trigger jailing on any unbond
        print(f"  Setting min_self_delegation to {actual_tokens}")
        rsp = cli.edit_validator(min_self_delegation=str(actual_tokens))
        assert rsp["code"] == 0, (
            f"Edit validator failed with code {rsp['code']}: "
            f"{rsp.get('raw_log', rsp)}"
        )
        print("  Successfully set min_self_delegation")
        wait_for_new_blocks(cli, 2)

    # Unbond from each validator to trigger the min_self_delegation check
    unbond_amount = "1000000000000000000stake"  # 1 stake token

    print("\n=== Phase 3: Unbonding to trigger min_self_delegation violation ===")

    for test_val_info in test_validators:
        cli = test_val_info["cli"]
        val_addr = test_val_info["val_addr"]
        node_idx = test_val_info["node_idx"]
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

        print(
            f"\nNode {node_idx} (cache-size={cache_size}): "
            f"Unbonding {unbond_amount} from validator"
        )
        rsp = cli.unbond_amount(val_addr, unbond_amount, "validator")

        assert rsp["code"] == 0, (
            f"Validator self-unbond returned code {rsp['code']}: "
            f"{rsp.get('raw_log', rsp)}"
        )
        print("  Unbonding transaction successful")
        wait_for_new_blocks(cli, 2)

    # Wait for 3 more blocks to ensure state propagation and jailing across all nodes
    cli = cronos.cosmos_cli()
    wait_for_new_blocks(cli, 3)

    # Check validator status on all nodes after unbonding
    print(
        "\n=== Phase 4: Validator status after unbonding " "(should be UNBONDING) ==="
    )

    for test_val_info in test_validators:
        val_addr = test_val_info["val_addr"]
        node_idx = test_val_info["node_idx"]

        print(f"\nChecking validator from node {node_idx}: {val_addr}")
        unbonding_statuses = []

        for check_node_idx in range(7):
            node_cli = cronos.cosmos_cli(check_node_idx)
            cache_size = [-1, 0, 1, 2, 3, 100, 1000][check_node_idx]
            validator = node_cli.validator(val_addr)

            assert validator and "validator" in validator, (
                f"Node {check_node_idx} (cache-size={cache_size}): "
                f"Failed to query validator {val_addr} after unbonding"
            )

            val_info = validator["validator"]
            status = val_info.get("status", "unknown")
            tokens = val_info.get("tokens", "0")
            jailed = val_info.get("jailed", False)
            unbonding_statuses.append(status)
            print(
                f"  Node {check_node_idx} (cache-size={cache_size}): "
                f"Status={status}, Tokens={tokens}, Jailed={jailed}"
            )

        # Assert validator is in BOND_STATUS_UNBONDING on all nodes
        assert len(set(unbonding_statuses)) == 1, (
            f"Validator has different statuses across nodes: " f"{unbonding_statuses}"
        )
        assert (
            unbonding_statuses[0] == "BOND_STATUS_UNBONDING"
        ), f"Expected BOND_STATUS_UNBONDING but got {unbonding_statuses[0]}"
        print("  ✓ Validator is in BOND_STATUS_UNBONDING on all nodes")

    # Wait for unbonding period to complete (60 seconds, 60 blocks)
    print("\n=== Phase 5: Waiting for unbonding period (60 seconds) ===")
    wait_for_new_blocks(cli, 60)

    # Check validator count after unbonding period
    print("\n=== Phase 6: Checking validator count " "after unbonding period ===")
    initial_validator_count = 7  # We started with 7 validators
    validator_counts = []

    for node_idx in range(7):
        node_cli = cronos.cosmos_cli(node_idx)
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]
        validators = node_cli.validators()
        count = len(validators)
        validator_counts.append(count)
        msg = f"Node {node_idx} (cache-size={cache_size}): " f"{count} validators"
        print(msg)

    # Assert all nodes have consistent validator count
    assert (
        len(set(validator_counts)) == 1
    ), f"Nodes have different validator counts: {validator_counts}"

    # Assert validator count reduced by 3 (we unbonded 3 validators)
    expected_count = initial_validator_count - 3
    assert validator_counts[0] == expected_count, (
        f"Expected {expected_count} validators but got " f"{validator_counts[0]}"
    )
    msg = f"✓ All nodes consistent: {validator_counts[0]} " f"validators (reduced by 3)"
    print(msg)

    print("\n=== Test completed successfully ===")


def test_staking_cache_consistency(cronos_staking_cache):
    """
    Comprehensive test combining delegations, redelegations, and unbonding operations.

    This test performs a complete lifecycle of staking operations and verifies
    consistency across all nodes before, during, and after the unbonding period:
    1. Initial delegations with consistency checks
    2. Redelegations with consistency checks (before maturation)
    3. Unbonding delegations with consistency checks (before maturation)
    4. Validator unbonding with status verification
    5. Wait for unbonding period
    6. Verify all unbonding delegations and redelegations have matured (empty)
    """
    cronos = cronos_staking_cache
    cli = cronos.cosmos_cli()

    print("\n=== Comprehensive Staking Cache Consistency Test ===")

    # Get all validator addresses
    validators = []
    for i in range(7):
        val_addr = cronos.cosmos_cli(i).address("validator", bech="val")
        validators.append(val_addr)

    print(f"Validators: {validators}")

    # Get delegator addresses
    rich_addr = cli.address("rich")
    alice_addr = cli.address("alice")
    bob_addr = cli.address("bob")
    charlie_addr = cli.address("charlie")

    # ========== PHASE 1: Initial Delegations ==========
    print("\n=== Phase 1: Initial delegations ===")
    delegation_amount = "2000000000000000000stake"  # 2 stake tokens

    # Rich delegates to validators 0 and 1
    print("Rich delegating to validators 0, 1")
    for i in range(2):
        print(f"  Delegating to validator {i}")
        rsp = cli.delegate_amount(validators[i], delegation_amount, "rich")
        assert rsp["code"] == 0, f"Rich delegation failed: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

    # Alice delegates to validators 1 and 2
    print("Alice delegating to validators 1, 2")
    for i in range(1, 3):
        print(f"  Delegating to validator {i}")
        rsp = cli.delegate_amount(validators[i], delegation_amount, "alice")
        assert rsp["code"] == 0, f"Alice delegation failed: {rsp.get('raw_log', rsp)}"
        wait_for_new_blocks(cli, 2)

    # Bob delegates to validator 3
    print("Bob delegating to validator 3")
    rsp = cli.delegate_amount(validators[3], delegation_amount, "bob")
    assert rsp["code"] == 0, f"Bob delegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Charlie delegates to validator 0
    print("Charlie delegating to validator 0")
    rsp = cli.delegate_amount(validators[0], delegation_amount, "charlie")
    assert rsp["code"] == 0, f"Charlie delegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Verify delegations consistency
    print("\n=== Verifying initial delegations consistency ===")
    for delegator_name, delegator_addr in [
        ("Rich", rich_addr),
        ("Alice", alice_addr),
        ("Bob", bob_addr),
        ("Charlie", charlie_addr),
    ]:
        delegation_counts = []
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            delegations = node_cli.get_delegated_amount(delegator_addr)
            delegation_responses = (
                delegations.get("delegation_responses", []) if delegations else []
            )
            count = len(delegation_responses)
            delegation_counts.append(count)

        assert len(set(delegation_counts)) == 1, (
            f"{delegator_name}'s delegations inconsistent "
            f"across nodes: {delegation_counts}"
        )
        msg = f"✓ {delegator_name}: {delegation_counts[0]} delegations across all nodes"
        print(msg)

    # ========== PHASE 2: Redelegations ==========
    print("\n=== Phase 2: Redelegations ===")
    redelegate_amount = "1000000000000000000stake"  # 1 stake token

    # Rich: Redelegate from validator 0 to validator 2
    print("Rich redelegating from validator 0 to validator 2")
    rsp = cli.redelegate_amount(validators[2], validators[0], redelegate_amount, "rich")
    assert rsp["code"] == 0, f"Rich redelegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Charlie: Redelegate from validator 0 to validator 3
    print("Charlie redelegating from validator 0 to validator 3")
    rsp = cli.redelegate_amount(
        validators[3], validators[0], redelegate_amount, "charlie"
    )
    assert rsp["code"] == 0, f"Charlie redelegation failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Verify redelegations consistency (before maturation)
    print("\n=== Verifying redelegations consistency (before maturation) ===")
    expected_redelegations = [
        (validators[0], validators[2], rich_addr, "Rich"),
        (validators[0], validators[3], charlie_addr, "Charlie"),
    ]

    for src_val, dst_val, delegator_addr, delegator_name in expected_redelegations:
        redelegation_counts = []
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            redelegations = node_cli.get_redelegations(delegator_addr, src_val, dst_val)
            count = len(redelegations) if redelegations else 0
            redelegation_counts.append(count)

        assert len(set(redelegation_counts)) == 1, (
            f"{delegator_name}'s redelegation inconsistent: " f"{redelegation_counts}"
        )
        assert redelegation_counts[0] == 1, (
            f"{delegator_name}'s redelegation expected 1 entry "
            f"but got {redelegation_counts[0]}"
        )
        msg = (
            f"✓ {delegator_name}: {redelegation_counts[0]} "
            f"redelegation across all nodes"
        )
        print(msg)

    # ========== PHASE 3: Unbonding Delegations ==========
    print("\n=== Phase 3: Unbonding delegations ===")
    unbond_amount = "500000000000000000stake"  # 0.5 stake token

    # Alice unbonds from validator 1
    print("Alice unbonding from validator 1")
    rsp = cli.unbond_amount(validators[1], unbond_amount, "alice")
    assert rsp["code"] == 0, f"Alice unbonding failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Bob unbonds from validator 3
    print("Bob unbonding from validator 3")
    rsp = cli.unbond_amount(validators[3], unbond_amount, "bob")
    assert rsp["code"] == 0, f"Bob unbonding failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cli, 2)

    # Verify unbonding delegations consistency (before maturation)
    print("\n=== Verifying unbonding delegations consistency (before maturation) ===")
    unbonding_delegators = [("Alice", alice_addr), ("Bob", bob_addr)]

    for delegator_name, delegator_addr in unbonding_delegators:
        unbonding_counts = []
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            unbonding = node_cli.get_unbonding_delegations(delegator_addr)
            count = len(unbonding) if unbonding else 0
            unbonding_counts.append(count)

        assert (
            len(set(unbonding_counts)) == 1
        ), f"{delegator_name}'s unbonding inconsistent: {unbonding_counts}"
        msg = (
            f"✓ {delegator_name}: {unbonding_counts[0]} "
            f"unbonding delegations across all nodes"
        )
        print(msg)

    # ========== PHASE 4: Validator Unbonding ==========
    print("\n=== Phase 4: Validator unbonding (3 validators) ===")

    # Use validators 4, 5, and 6 for unbonding test
    test_node_indices = [4, 5, 6]
    test_validators = []

    # Get initial validator count
    initial_validators = cli.validators()
    initial_count = len(initial_validators)
    print(f"Initial validator count: {initial_count}")

    # Collect validator info for all test nodes
    for test_node_idx in test_node_indices:
        val_cli = cronos.cosmos_cli(test_node_idx)
        val_addr = val_cli.address("validator", bech="val")
        test_validators.append(
            {"node_idx": test_node_idx, "cli": val_cli, "val_addr": val_addr}
        )
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][test_node_idx]
        print(
            f"Will unbond validator from node {test_node_idx} "
            f"(cache-size={cache_size}): {val_addr}"
        )

    # Query each validator's actual total tokens and set min_self_delegation
    print("\nQuerying validators' actual tokens and setting min_self_delegation")

    for test_val_info in test_validators:
        val_cli = test_val_info["cli"]
        val_addr = test_val_info["val_addr"]
        node_idx = test_val_info["node_idx"]
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

        print(
            f"\nNode {node_idx} (cache-size={cache_size}): "
            f"Processing validator {val_addr}"
        )

        validator_info = val_cli.validator(val_addr)
        assert (
            validator_info and "validator" in validator_info
        ), f"Failed to query validator {val_addr}"

        actual_tokens = int(validator_info["validator"].get("tokens", "0"))
        print(f"  Validator's actual total tokens: {actual_tokens}")
        test_val_info["actual_tokens"] = actual_tokens

        # Set min_self_delegation to current tokens to trigger jailing on any unbond
        print(f"  Setting min_self_delegation to {actual_tokens}")
        rsp = val_cli.edit_validator(min_self_delegation=str(actual_tokens))
        assert rsp["code"] == 0, (
            f"Edit validator failed with code {rsp['code']}: "
            f"{rsp.get('raw_log', rsp)}"
        )
        print("  Successfully set min_self_delegation")
        wait_for_new_blocks(cli, 2)

    # Unbond from each validator to trigger the min_self_delegation check
    unbond_val_amount = "1000000000000000000stake"  # 1 stake token
    print(
        f"\nUnbonding {unbond_val_amount} from each validator to trigger "
        f"min_self_delegation violation"
    )

    for test_val_info in test_validators:
        val_cli = test_val_info["cli"]
        val_addr = test_val_info["val_addr"]
        node_idx = test_val_info["node_idx"]
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]

        print(f"\nNode {node_idx} (cache-size={cache_size}): Unbonding from validator")
        rsp = val_cli.unbond_amount(val_addr, unbond_val_amount, "validator")
        assert (
            rsp["code"] == 0
        ), f"Validator unbonding failed: {rsp.get('raw_log', rsp)}"
        print("  Unbonding transaction successful")
        wait_for_new_blocks(cli, 2)

    # Wait for 3 more blocks to ensure state propagation and jailing across all nodes
    wait_for_new_blocks(cli, 3)

    # Verify validators are in UNBONDING status across all nodes
    print("\n=== Verifying validator UNBONDING status ===")

    for test_val_info in test_validators:
        val_addr = test_val_info["val_addr"]
        node_idx = test_val_info["node_idx"]

        print(f"\nChecking validator from node {node_idx}: {val_addr}")
        unbonding_statuses = []

        for check_node_idx in range(7):
            node_cli = cronos.cosmos_cli(check_node_idx)
            cache_size = [-1, 0, 1, 2, 3, 100, 1000][check_node_idx]
            validator = node_cli.validator(val_addr)

            assert validator and "validator" in validator, (
                f"Node {check_node_idx} (cache-size={cache_size}): "
                f"Failed to query validator {val_addr} after unbonding"
            )

            val_info = validator["validator"]
            status = val_info.get("status", "unknown")
            unbonding_statuses.append(status)
            print(f"  Node {check_node_idx} (cache-size={cache_size}): Status={status}")

        assert (
            len(set(unbonding_statuses)) == 1
        ), f"Validator has different statuses across nodes: {unbonding_statuses}"
        assert (
            unbonding_statuses[0] == "BOND_STATUS_UNBONDING"
        ), f"Expected BOND_STATUS_UNBONDING but got {unbonding_statuses[0]}"
        print("  ✓ Validator is in BOND_STATUS_UNBONDING on all nodes")

    # ========== PHASE 5: Wait for Unbonding Period ==========
    print("\n=== Phase 5: Waiting for unbonding period (60 blocks ≈ 60 seconds) ===")
    wait_for_new_blocks(cli, 60)

    # ========== PHASE 6: Verify All Matured ==========
    print("\n=== Phase 6: Verifying all operations matured ===")

    # Check redelegations are now empty
    print("\n--- Checking redelegations matured (should be empty) ---")
    for src_val, dst_val, delegator_addr, delegator_name in expected_redelegations:
        matured_counts = []
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            try:
                redelegations = node_cli.get_redelegations(
                    delegator_addr, src_val, dst_val
                )
                count = len(redelegations) if redelegations else 0
                matured_counts.append(count)
            except Exception as e:
                # "not found" errors are expected when redelegations have matured
                error_str = str(e).lower()
                if "not found" in error_str:
                    matured_counts.append(0)
                else:
                    # Unexpected error - fail the test
                    raise

        assert (
            len(set(matured_counts)) == 1
        ), f"{delegator_name}'s matured redelegation inconsistent: {matured_counts}"
        assert matured_counts[0] == 0, (
            f"{delegator_name}'s redelegation expected 0 "
            f"after maturation but got {matured_counts[0]}"
        )
        print(f"✓ {delegator_name}: 0 redelegations (matured)")

    # Check unbonding delegations are now empty
    print("\n--- Checking unbonding delegations matured (should be empty) ---")
    for delegator_name, delegator_addr in unbonding_delegators:
        matured_counts = []
        for node_idx in range(7):
            node_cli = cronos.cosmos_cli(node_idx)
            unbonding = node_cli.get_unbonding_delegations(delegator_addr)
            count = len(unbonding) if unbonding else 0
            matured_counts.append(count)

        assert (
            len(set(matured_counts)) == 1
        ), f"{delegator_name}'s matured unbonding inconsistent: {matured_counts}"
        assert matured_counts[0] == 0, (
            f"{delegator_name}'s unbonding expected 0 "
            f"after maturation but got {matured_counts[0]}"
        )
        print(f"✓ {delegator_name}: 0 unbonding delegations (matured)")

    # Check validator count reduced by 3
    print("\n--- Checking validator count after unbonding period ---")
    validator_counts = []
    for node_idx in range(7):
        node_cli = cronos.cosmos_cli(node_idx)
        cache_size = [-1, 0, 1, 2, 3, 100, 1000][node_idx]
        validators_list = node_cli.validators()
        count = len(validators_list)
        validator_counts.append(count)
        print(f"  Node {node_idx} (cache-size={cache_size}): {count} validators")

    assert (
        len(set(validator_counts)) == 1
    ), f"Nodes have different validator counts: {validator_counts}"
    expected_count = initial_count - 3
    assert (
        validator_counts[0] == expected_count
    ), f"Expected {expected_count} validators but got {validator_counts[0]}"
    msg = f"✓ All nodes consistent: {validator_counts[0]} validators (reduced by 3)"
    print(msg)

    print("\n=== Test completed successfully ===")
