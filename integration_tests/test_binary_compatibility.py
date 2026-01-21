"""
Integration test for binary compatibility testing.

Tests whether nodes running different binaries can work together.
Test fails (BREAKING CHANGE) if any node stops progressing blocks.

- Compatible: All nodes continue producing blocks
- Breaking: Any node(s) stuck - test fails with error details and log locations

Binaries are defined in configs/binary-compat-package.nix
"""

import re
import shutil
import stat
import subprocess
import time
from contextlib import contextmanager
from pathlib import Path

import pytest
from pystarport import ports

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    edit_ini_sections,
    send_transaction,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.binary_compatibility


def setup_binary_compatibility_test_nix(
    tmp_path_factory, nix_package="binary-compat-package"
):
    """
    Setup binary compatibility test using Nix-built binaries.

    Args:
        tmp_path_factory: pytest tmp_path_factory fixture
        nix_package: Name of the Nix package file (without .nix extension)

    Returns:
        tuple: (cronos instance, binaries_path)
    """
    path = tmp_path_factory.mktemp("binary-compat")
    port = 26300
    configdir = Path(__file__).parent

    # Build the Nix package containing both binaries
    nix_file = configdir / f"configs/{nix_package}.nix"
    nix_result = path / "result"
    print(f"Building Nix package: {nix_file}")
    cmd = ["nix-build", nix_file, "--out-link", str(nix_result)]
    print(*cmd)
    subprocess.run(cmd, check=True)

    # Copy the binaries directory
    binaries = path / "binaries"
    shutil.copytree(nix_result, binaries)
    mod = stat.S_IRWXU
    binaries.chmod(mod)
    for d in binaries.iterdir():
        d.chmod(mod)

    # Get binary paths
    initial_binary_path = str(binaries / "initial/bin/cronosd")
    new_binary_path = str(binaries / "new/bin/cronosd")

    print(f"Initial Binary: {initial_binary_path}")
    print(f"New Binary: {new_binary_path}")

    # Initialize all nodes with initial binary
    with contextmanager(setup_custom_cronos)(
        path,
        port,
        configdir / "configs/binary-compat.jsonnet",
        chain_binary=initial_binary_path,
    ) as cronos:
        yield cronos, binaries, initial_binary_path, new_binary_path


def get_node_status(cronos, node_index):
    """Get status of a specific node."""
    try:
        cli = cronos.cosmos_cli(node_index)
        status = cli.status()
        return status
    except Exception as e:
        print(f"Failed to get status for node {node_index}: {e}")
        return None


def check_consensus_failure(cronos):
    """
    Check if consensus failure errors appear in node logs.

    Returns:
        dict: {node_index: (has_error, error_message)}
    """
    chain_id = "cronos_777-1"
    results = {}

    for i, _ in enumerate(cronos.config["validators"]):
        log_file = cronos.base_dir / f"node{i}.log"

        if not log_file.exists():
            log_file = cronos.base_dir.parent / f"{chain_id}-node{i}.log"

        has_error = False
        error_msg = None

        if log_file.exists():
            try:
                with open(log_file, "r") as f:
                    lines = f.readlines()
                    # Check last 2000 lines for errors
                    recent_lines = lines[-2000:] if len(lines) > 2000 else lines
                    log_content = "".join(recent_lines)

                    # Check for various consensus failure patterns
                    patterns = [
                        (r"wrong Block\.Header\.AppHash", "App hash mismatch"),
                        (
                            r"wrong Block\.Header\.LastResultsHash",
                            "Last results hash mismatch",
                        ),
                        (r"wrong Block\.Header\.DataHash", "Data hash mismatch"),
                        (
                            r"wrong Block\.Header\.ValidatorsHash",
                            "Validators hash mismatch",
                        ),
                        (
                            r"wrong Block\.Header\.ConsensusHash",
                            "Consensus hash mismatch",
                        ),
                        (r"CONSENSUS FAILURE", "Consensus failure"),
                        (
                            r"consensus deems this block invalid",
                            "Block deemed invalid by consensus",
                        ),
                        (r"appHash.*expected.*got", "App hash mismatch"),
                    ]

                    for pattern, description in patterns:
                        match = re.search(pattern, log_content, re.IGNORECASE)
                        if match:
                            has_error = True
                            # Extract the full error line
                            error_line = match.group(0)
                            error_msg = f"{description}: {error_line[:100]}"
                            print(f"Node {i}: {error_msg}")
                            break
            except Exception as e:
                print(f"Error reading log file for node {i}: {e}")

        results[i] = (has_error, error_msg)

    return results


def check_block_progression(
    cronos, initial_heights, min_new_blocks=5, timeout=60, check_errors=True
):
    """
    Check if nodes are producing new blocks (checks all nodes in parallel).
    If blocks aren't progressing and check_errors=True, check for consensus errors.

    Returns:
        dict: {node_index: bool} indicating if node is progressing

    Raises:
        AssertionError: If blocks aren't progressing due to consensus errors
    """
    start_time = time.time()
    results = {node_index: False for node_index in initial_heights.keys()}
    last_error_check = start_time

    # Check all nodes in parallel instead of sequentially
    while time.time() - start_time < timeout:
        all_done = True

        for node_index in initial_heights.keys():
            if results[node_index]:
                continue  # Already progressed

            try:
                status = get_node_status(cronos, node_index)
                if status:
                    sync_info = status.get("SyncInfo") or status.get("sync_info")
                    new_height = int(sync_info["latest_block_height"])

                    if new_height >= initial_heights[node_index] + min_new_blocks:
                        results[node_index] = True
                        print(
                            f"Node {node_index}: Block height progressed from "
                            f"{initial_heights[node_index]} to {new_height}"
                        )
                    else:
                        all_done = False
            except Exception as e:
                print(f"Error checking block progression for node {node_index}: {e}")
                all_done = False

        # If all nodes that can progress have progressed, we can stop early
        if all_done:
            break

        # Check for consensus errors every 10 seconds if blocks aren't progressing
        if check_errors and time.time() - last_error_check > 10:
            consensus_errors = check_consensus_failure(cronos)
            error_nodes = [n for n, (has_err, _) in consensus_errors.items() if has_err]
            if error_nodes:
                error_msgs = [
                    f"Node {n}: {msg}"
                    for n, (has_err, msg) in consensus_errors.items()
                    if has_err and msg
                ]
                raise AssertionError(
                    "Consensus failure detected while waiting for blocks:\n"
                    + "\n".join(error_msgs)
                )
            last_error_check = time.time()

        time.sleep(2)

    return results


def get_compatibility_status(cronos, initial_heights, min_new_blocks=3, timeout=30):
    """
    Check compatibility status of all nodes.

    Returns:
        tuple: (progression_results, consensus_errors)
            - progression_results: dict of {node_index: bool}
            - consensus_errors: dict of {node_index: (has_error, error_message)}
    """
    # Check block progression
    progression_results = check_block_progression(
        cronos,
        initial_heights,
        min_new_blocks=min_new_blocks,
        timeout=timeout,
        check_errors=False,
    )

    # Check for consensus failures in logs
    consensus_errors = check_consensus_failure(cronos)

    return progression_results, consensus_errors


def check_for_breaking_change(
    cronos,
    initial_version,
    new_version,
    upgraded_nodes=None,
):
    """
    Check if all nodes are progressing.
    Raises AssertionError if breaking change detected.
    """
    if upgraded_nodes is None:
        upgraded_nodes = []

    total_nodes = len(cronos.config["validators"])
    non_upgraded_nodes = [i for i in range(total_nodes) if i not in upgraded_nodes]

    # Get current heights
    initial_heights = {}
    for i in range(total_nodes):
        status = get_node_status(cronos, i)
        if status:
            sync_info = status.get("SyncInfo") or status.get("sync_info")
            initial_heights[i] = int(sync_info["latest_block_height"])

    # Quick check with 5 second timeout
    progression_results, consensus_errors = get_compatibility_status(
        cronos, initial_heights, min_new_blocks=1, timeout=5
    )

    progressing_count = sum(1 for p in progression_results.values() if p)

    # If not all nodes progressing, it's a breaking change - fail the test
    if progressing_count < total_nodes:
        stuck_nodes = [i for i, p in progression_results.items() if not p]
        progressing_nodes = [i for i, p in progression_results.items() if p]

        error_details = []
        for node_idx in range(total_nodes):
            has_err, error_msg = consensus_errors.get(node_idx, (False, None))
            if has_err and error_msg:
                error_details.append(f"  Node {node_idx}: {error_msg}")

        error_summary = (
            "\n".join(error_details)
            if error_details
            else "  Check node logs for details"
        )

        # Breaking change detected - fail the test
        raise AssertionError(
            f"BREAKING CHANGE DETECTED - Binaries are incompatible:\n\n"
            f"Nodes stuck: {stuck_nodes}\n"
            f"Nodes progressing: "
            f"{progressing_nodes if progressing_nodes else 'None'}\n\n"
            f"Binary versions:\n"
            f"  - Initial Binary (nodes {','.join(map(str, non_upgraded_nodes))}): "
            f"{initial_version}\n"
            f"  - New Binary (nodes {','.join(map(str, upgraded_nodes))}): "
            f"{new_version}\n\n"
            f"Error details:\n{error_summary}"
        )


def upgrade_node(cronos, node_idx, new_binary_path, initial_version, new_version):
    """
    Upgrade a single node to the new binary.

    Args:
        cronos: Cronos instance
        node_idx: Index of the node to upgrade
        new_binary_path: Path to the new binary
        initial_version: Version string of initial binary
        new_version: Version string of new binary
    """
    chain_id = "cronos_777-1"
    data = cronos.base_dir
    node_name = f"{chain_id}-node{node_idx}"

    print(f"\n{'='*60}")
    print(f"Upgrading node {node_idx} to new binary...")
    print(f"{'='*60}\n")

    # Update supervisor configuration for this node
    def update_command(i, old):
        if int(i) == node_idx:
            print(f"Updating config for node {i}\n")
            return {
                "command": f"{new_binary_path} start --home %(here)s/node{i}",
            }
        else:
            # Return empty dict for nodes that shouldn't be updated
            return {}

    edit_ini_sections(
        chain_id,
        data / "tasks.ini",
        update_command,
    )

    # Stop the node
    print(f"Stopping node {node_idx}...\n")
    cronos.supervisorctl("stop", node_name)
    print()

    # Reload supervisor configuration
    print("Reloading supervisor configuration...\n")
    cronos.supervisorctl("update")
    print()

    # Start the node with new binary
    print(f"Starting node {node_idx} with new binary...\n")
    cronos.supervisorctl("start", node_name)
    print()

    # Wait for node to restart
    print("Waiting for node to restart...\n")
    time.sleep(10)
    print()

    # Check if all nodes are still progressing
    print(f"Verifying node {node_idx} can sync with the network...\n")
    check_for_breaking_change(
        cronos,
        initial_version,
        new_version,
        [node_idx],
    )
    print(f"✓ Node {node_idx} successfully upgraded and syncing\n")


def run_transactions(cronos, phase_name, initial_version, new_version, upgraded_nodes):
    """
    Run a set of transactions to test the network.

    Args:
        cronos: Cronos instance
        phase_name: Name of the test phase (for logging)
        initial_version: Version of initial binary (for error reporting)
        new_version: Version of new binary (for error reporting)
        upgraded_nodes: List of node indices that have been upgraded
    """
    print(f"\n{'='*60}")
    print(f"Testing transactions - {phase_name}")
    print(f"{'='*60}\n")

    cli = cronos.cosmos_cli()
    w3 = cronos.w3

    # 1. Test MsgSend (cosmos transaction)
    print("1. Testing MsgSend transaction...")
    try:
        sender = "community"
        receiver = cli.address("validator")
        amount = "1000basetcro"
        rsp = cli.transfer(sender, receiver, amount)
        assert rsp["code"] == 0, f"MsgSend failed: {rsp.get('raw_log', '')}"
        print(f"   ✓ MsgSend successful (tx: {rsp['txhash'][:16]}...)")
    except Exception as e:
        print(f"   ✗ MsgSend failed: {e}")
        raise

    # Wait for transaction to be processed
    try:
        wait_for_new_blocks(cli, 2, 0.5, 10)
    except TimeoutError:
        print("   ⚠ Timeout waiting for blocks after MsgSend")
        print("      Checking for breaking change...")
        check_for_breaking_change(cronos, initial_version, new_version, upgraded_nodes)
        raise

    # 2. Test EVM transaction (simple transfer)
    print("2. Testing EVM transfer...")
    try:
        receipt = send_transaction(
            w3,
            {
                "to": ADDRS["community"],
                "value": 1000,
                "maxFeePerGas": 10000000000000,
                "maxPriorityFeePerGas": 10000,
            },
        )
        assert receipt.status == 1, "EVM transfer failed"
        print(f"   ✓ EVM transfer successful (block: {receipt.blockNumber})")
    except Exception as e:
        print(f"   ✗ EVM transfer failed: {e}")
        raise

    # Wait for transaction to be processed
    try:
        wait_for_new_blocks(cli, 2, 0.5, 5)
    except TimeoutError:
        print("   ⚠ Timeout waiting for blocks after EVM transfer")
        print("      Checking for breaking change...")
        check_for_breaking_change(cronos, initial_version, new_version, upgraded_nodes)
        raise

    # 3. Test contract deployment
    print("3. Testing contract deployment...")
    try:
        greeter = deploy_contract(w3, CONTRACTS["Greeter"])
        print(f"   ✓ Contract deployed at: {greeter.address}")

        # 4. Test contract interaction
        print("4. Testing contract interaction...")
        # Read initial greeting
        initial_greeting = greeter.caller.greet()
        print(f"   Initial greeting: {initial_greeting}")

        # Update greeting
        new_greeting = f"Hello from {phase_name}!"
        tx = greeter.functions.setGreeting(new_greeting).build_transaction(
            {
                "from": ADDRS["validator"],
                "maxFeePerGas": 10000000000000,
                "maxPriorityFeePerGas": 10000,
            }
        )
        receipt = send_transaction(w3, tx)
        assert receipt.status == 1, "Contract interaction failed"
        print(f"   ✓ Greeting updated (block: {receipt.blockNumber})")

        # Verify greeting was updated
        updated_greeting = greeter.caller.greet()
        assert updated_greeting == new_greeting, "Greeting not updated correctly"
        print(f"   ✓ Verified: {updated_greeting}")

    except Exception as e:
        print(f"   ✗ Contract operations failed: {e}")
        raise

    print(f"\n✓ All transactions successful for {phase_name}\n")


def test_binary_compatibility(tmp_path_factory):
    """
    Test binary compatibility using a rolling upgrade approach.

    The binaries are defined in configs/binary-compat-package.nix.

    Test Flow (Rolling Upgrade):
    1. All nodes start with initial binary
    2. Upgrade node 0, test transactions
    3. Upgrade node 1, test transactions
    4. Upgrade node 2, test transactions
    5. Final verification that all nodes continue to progress

    Test Logic:
    - ALL nodes must progress for binaries to be considered compatible
    - If any node is stuck at any stage, it's a BREAKING CHANGE (test fails immediately)
    - Transactions are tested after each individual node upgrade
    """
    generator = setup_binary_compatibility_test_nix(tmp_path_factory)
    for cronos, binaries, initial_binary_path, new_binary_path in generator:
        total_nodes = len(cronos.config["validators"])

        print(f"\n{'='*60}")
        print("Testing Binary Compatibility (Rolling Upgrade)")
        print("Binaries from: configs/binary-compat-package.nix")
        print(f"{'='*60}\n")

        # Show binary versions
        initial_version = subprocess.run(
            [str(binaries / "initial/bin/cronosd"), "version"],
            capture_output=True,
            text=True,
        ).stdout.strip()
        new_version = subprocess.run(
            [str(binaries / "new/bin/cronosd"), "version"],
            capture_output=True,
            text=True,
        ).stdout.strip()

        print(f"Initial Binary version: {initial_version}")
        print(f"New Binary version: {new_version}")
        print(f"Total nodes: {total_nodes}")
        print("Upgrade order: node 0 → node 1 → node 2\n")

        cli = cronos.cosmos_cli()

        # Wait for network to start
        wait_for_port(ports.evmrpc_port(cronos.base_port(0)))
        time.sleep(5)

        # Try to wait for initial blocks
        try:
            wait_for_new_blocks(cli, 3, timeout=30)
        except Exception as e:
            print(f"Warning: Could not wait for initial blocks: {e}")

        # Get initial heights
        initial_heights = {}
        for i in range(total_nodes):
            status = get_node_status(cronos, i)
            if status:
                sync_info = status.get("SyncInfo") or status.get("sync_info")
                initial_heights[i] = int(sync_info["latest_block_height"])

        print(f"Initial heights: {initial_heights}")
        print("\nAll nodes starting with initial binary...\n")

        # Rolling upgrade: upgrade each node in sequence and test after each
        upgraded_nodes = []
        for node_idx in range(total_nodes):
            # Upgrade the node
            upgrade_node(
                cronos, node_idx, new_binary_path, initial_version, new_version
            )
            upgraded_nodes.append(node_idx)

            # Test transactions after this node upgrade
            run_transactions(
                cronos,
                f"After node {node_idx} upgrade",
                initial_version,
                new_version,
                upgraded_nodes,
            )

        # Final comprehensive check - all nodes upgraded
        print("\n" + "=" * 60)
        print("Final Compatibility Check - All Nodes Upgraded")
        print("=" * 60)

        check_for_breaking_change(
            cronos,
            initial_version,
            new_version,
            upgraded_nodes,
        )

        # If we reach here, no breaking change detected - test passes
        print("\n✓ ROLLING UPGRADE SUCCESSFUL!")
        print(
            f"  All {total_nodes} nodes successfully upgraded from "
            f"{initial_version} to {new_version}"
        )
        print(
            "  The binaries are compatible and the rolling upgrade "
            "completed successfully\n"
        )


if __name__ == "__main__":
    import sys

    pytest.main([__file__, "-v", "-s"] + sys.argv[1:])
