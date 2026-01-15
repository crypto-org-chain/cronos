"""
Integration test for binary compatibility testing.

Tests whether nodes running different binaries can work together.
Test fails (BREAKING CHANGE) if any node stops progressing blocks.

- Compatible: All nodes continue producing blocks
- Breaking: Any node(s) stuck - test fails with error details and log locations

Binaries are defined in configs/binary-compat-package.nix
"""

import json
import os
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

pytestmark = pytest.mark.upgrade


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
    print(f"Building Nix package: {nix_file}")
    cmd = ["nix-build", nix_file]
    print(*cmd)
    subprocess.run(cmd, check=True)

    # Copy the binaries directory
    binaries = path / "binaries"
    shutil.copytree("./result", binaries)
    mod = stat.S_IRWXU
    binaries.chmod(mod)
    for d in binaries.iterdir():
        d.chmod(mod)

    # Get binary paths
    binary1_path = str(binaries / "binary1/bin/cronosd")
    binary2_path = str(binaries / "binary2/bin/cronosd")

    print(f"Binary 1: {binary1_path}")
    print(f"Binary 2: {binary2_path}")

    # Get node configuration from environment (default: node 2 runs binary2)
    nodes_with_binary2_str = os.environ.get("NODES_WITH_BINARY2", "2")
    nodes_with_binary2 = [
        int(x.strip()) for x in nodes_with_binary2_str.split(",")
        if x.strip()
    ]

    def post_init(path, base_port, config):
        """Configure supervisor to run different binaries on different nodes."""
        chain_id = "cronos_777-1"
        data = path / chain_id

        def update_command(i, old):
            node_index = int(i)
            if node_index in nodes_with_binary2:
                binary = binary2_path
            else:
                binary = binary1_path

            return {
                "command": f"{binary} start --home %(here)s/node{i}",
            }

        edit_ini_sections(
            chain_id,
            data / "tasks.ini",
            update_command,
        )

    # Initialize with binary1 (genesis binary)
    with contextmanager(setup_custom_cronos)(
        path,
        port,
        configdir / "configs/binary-compat.jsonnet",
        post_init=post_init,
        chain_binary=binary1_path,
    ) as cronos:
        yield cronos, binaries


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
                        (r"wrong Block\.Header\.AppHash",
                         "App hash mismatch"),
                        (r"wrong Block\.Header\.LastResultsHash",
                         "Last results hash mismatch"),
                        (r"wrong Block\.Header\.DataHash",
                         "Data hash mismatch"),
                        (r"wrong Block\.Header\.ValidatorsHash",
                         "Validators hash mismatch"),
                        (r"wrong Block\.Header\.ConsensusHash",
                         "Consensus hash mismatch"),
                        (r"CONSENSUS FAILURE", "Consensus failure"),
                        (r"consensus deems this block invalid",
                         "Block deemed invalid by consensus"),
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


def get_node_status(cronos, node_index):
    """Get status of a specific node."""
    try:
        cli = cronos.cosmos_cli(node_index)
        status = cli.status()
        return status
    except Exception as e:
        print(f"Failed to get status for node {node_index}: {e}")
        return None


def get_compatibility_status(
    cronos, initial_heights, min_new_blocks=3, timeout=30
):
    """
    Check compatibility status of all nodes.

    Returns:
        tuple: (progression_results, consensus_errors)
            - progression_results: dict of {node_index: bool}
            - consensus_errors: dict of {node_index: (has_error, error_message)}
    """
    # Check block progression
    progression_results = check_block_progression(
        cronos, initial_heights, min_new_blocks=min_new_blocks,
        timeout=timeout, check_errors=False
    )

    # Check for consensus failures in logs
    consensus_errors = check_consensus_failure(cronos)

    return progression_results, consensus_errors


def display_compatibility_results(
    cronos, progression_results, consensus_errors,
    binary1_version, binary2_version
):
    """Display compatibility test results in a formatted table."""
    progressing_count = sum(1 for p in progression_results.values() if p)
    total_nodes = len(cronos.config["validators"])

    print(f"\n{'='*60}")
    print("Binary Compatibility Test Results")
    print(f"{'='*60}")
    print(f"Binary 1 (nodes 0,1): {binary1_version}")
    print(f"Binary 2 (node 2): {binary2_version}")
    print(f"Nodes progressing: {progressing_count}/{total_nodes}")
    print("\nNode Status:")
    for idx in range(total_nodes):
        progressed = progression_results.get(idx, False)
        has_err, error_msg = consensus_errors.get(idx, (False, None))

        status = "✓ PROGRESSING" if progressed else "✗ STUCK"
        print(f"  Node {idx}: {status}")
        if has_err and error_msg:
            print(f"    Error: {error_msg}")
    print(f"{'='*60}\n")

    return progressing_count, total_nodes


def check_for_breaking_change(cronos, binary1_version, binary2_version):
    """
    Quick check for breaking change (used during test execution).
    Raises AssertionError immediately if breaking change detected.
    """
    # Get current heights
    initial_heights = {}
    for i in range(len(cronos.config["validators"])):
        status = get_node_status(cronos, i)
        if status:
            sync_info = status.get("SyncInfo") or status.get("sync_info")
            initial_heights[i] = int(sync_info["latest_block_height"])

    # Quick check with 5 second timeout
    progression_results, consensus_errors = get_compatibility_status(
        cronos, initial_heights, min_new_blocks=1, timeout=5
    )

    progressing_count = sum(1 for p in progression_results.values() if p)
    total_nodes = len(cronos.config["validators"])

    # If not all nodes progressing, it's breaking
    if progressing_count < total_nodes:
        stuck_nodes = [i for i, p in progression_results.items() if not p]
        progressing_nodes = [i for i, p in progression_results.items() if p]

        error_details = []
        for node_idx in range(total_nodes):
            has_err, error_msg = consensus_errors.get(node_idx, (False, None))
            if has_err and error_msg:
                error_details.append(f"  Node {node_idx}: {error_msg}")

        error_summary = (
            "\n".join(error_details) if error_details
            else "  Check node logs for details"
        )

        raise AssertionError(
            f"BREAKING CHANGE DETECTED during test execution:\n\n"
            f"Nodes stuck: {stuck_nodes}\n"
            f"Nodes progressing: "
            f"{progressing_nodes if progressing_nodes else 'None'}\n\n"
            f"Binary versions:\n"
            f"  - Binary 1 (nodes 0,1): {binary1_version}\n"
            f"  - Binary 2 (node 2): {binary2_version}\n\n"
            f"Error details:\n{error_summary}\n\n"
            f"The binaries are incompatible. Test cannot continue."
        )


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
                    "Consensus failure detected while waiting for blocks:\n" +
                    "\n".join(error_msgs)
                )
            last_error_check = time.time()

        time.sleep(2)

    return results


def test_binary_compatibility(tmp_path_factory):
    """
    Test binary compatibility using Nix-built binaries.

    The binaries and test expectations are defined in configs/binary-compat-package.nix.

    Configuration:
    - Nodes 0, 1: Run binary1 (2/3 voting power)
    - Node 2: Runs binary2 (1/3 voting power)

    Test Logic:
    - ALL nodes must progress for binaries to be considered compatible
    - If any node is stuck, it's a BREAKING CHANGE (test fails with details)
    - expect_breaking: Controls whether the test expects a breaking change or not

    To customize which nodes run which binary:
        export NODES_WITH_BINARY2="2"    # default
        export NODES_WITH_BINARY2="0,1"  # nodes 0,1 run binary2
    """
    for cronos, binaries in setup_binary_compatibility_test_nix(tmp_path_factory):
        # Read test configuration from Nix build
        config_file = binaries / "config.json"
        with open(config_file) as f:
            config = json.load(f)
        expect_breaking = config.get("expect_breaking", False)

        print(f"\n{'='*60}")
        print("Testing Binary Compatibility")
        print(f"Expected: {'BREAKING' if expect_breaking else 'NON-BREAKING'}")
        print("Binaries from: configs/binary-compat-package.nix")
        print(f"{'='*60}\n")

        # Show binary versions
        binary1_version = subprocess.run(
            [str(binaries / "binary1/bin/cronosd"), "version"],
            capture_output=True, text=True
        ).stdout.strip()
        binary2_version = subprocess.run(
            [str(binaries / "binary2/bin/cronosd"), "version"],
            capture_output=True, text=True
        ).stdout.strip()

        print(f"Binary 1 version: {binary1_version}")
        print(f"Binary 2 version: {binary2_version}")

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
        for i in range(len(cronos.config["validators"])):
            status = get_node_status(cronos, i)
            if status:
                sync_info = status.get("SyncInfo") or status.get("sync_info")
                initial_heights[i] = int(sync_info["latest_block_height"])

        print(f"Initial heights: {initial_heights}")

        # Execute transactions to test state divergence
        print("\n" + "="*60)
        print("Executing transactions to test state consistency...")
        print("="*60 + "\n")

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
            if expect_breaking:
                print("   (Expected for breaking change)")

        # Wait for transaction to be processed
        try:
            wait_for_new_blocks(cli, 2, 0.5, 10)
        except TimeoutError:
            print("   ⚠ Timeout waiting for blocks - checking for breaking change...")
            check_for_breaking_change(cronos, binary1_version, binary2_version)

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
            if expect_breaking:
                print("   (Expected for breaking change)")

        # Wait for transaction to be processed
        try:
            wait_for_new_blocks(cli, 2, 0.5, 5)
        except TimeoutError:
            print("   ⚠ Timeout waiting for blocks - checking for breaking change...")
            check_for_breaking_change(cronos, binary1_version, binary2_version)

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
            new_greeting = "Hello from compatibility test!"
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
            if expect_breaking:
                print("   (Expected for breaking change)")

        # Wait for all transactions to be processed
        print("\nWaiting for transactions to be fully processed...")
        try:
            wait_for_new_blocks(cli, 3, 0.5, 5)
        except TimeoutError:
            print("   ⚠ Timeout waiting for blocks - checking for breaking change...")
            check_for_breaking_change(cronos, binary1_version, binary2_version)

        # Check block progression and consensus errors
        progression_results, consensus_errors = get_compatibility_status(
            cronos, initial_heights, min_new_blocks=3, timeout=30
        )

        # Display detailed results
        progressing_count, total_nodes = display_compatibility_results(
            cronos, progression_results, consensus_errors,
            binary1_version, binary2_version
        )

        # If not all nodes are progressing, it's a breaking change
        if progressing_count < total_nodes:
            stuck_nodes = [i for i, p in progression_results.items() if not p]
            progressing_nodes = [i for i, p in progression_results.items() if p]

            error_details = []
            for node_idx in range(total_nodes):
                has_err, error_msg = consensus_errors.get(node_idx, (False, None))
                if has_err and error_msg:
                    error_details.append(f"  Node {node_idx}: {error_msg}")

            error_summary = (
                "\n".join(error_details) if error_details
                else "  No specific error patterns found in logs "
                "(check node logs for details)"
            )

            pytest.fail(
                f"BREAKING CHANGE DETECTED: Not all nodes are progressing\n\n"
                f"Nodes stuck: {stuck_nodes}\n"
                f"Nodes progressing: "
                f"{progressing_nodes if progressing_nodes else 'None'}\n\n"
                f"Binary versions:\n"
                f"  - Binary 1 (nodes 0,1): {binary1_version}\n"
                f"  - Binary 2 (node 2): {binary2_version}\n\n"
                f"Error details:\n{error_summary}\n\n"
                f"The binaries are incompatible and cannot work together.\n"
                f"Check node logs for more details:\n"
                f"  - {cronos.base_dir}/../cronos_777-1-node*.log\n\n"
                f"If this breaking change is expected, set expect_breaking=true in:\n"
                f"  configs/binary-compat-package.nix"
            )

        # All nodes progressing - binaries are compatible!
        print(f"✓ COMPATIBLE: All {total_nodes} nodes are progressing successfully")
        print("  The binaries are compatible and can work together\n")

        # If we expected breaking but got compatible, fail the test
        if expect_breaking:
            pytest.fail(
                f"Expected breaking change but binaries are compatible:\n"
                f"  - All {total_nodes} nodes are progressing normally\n"
                f"  - Binary 1 (nodes 0,1): {binary1_version}\n"
                f"  - Binary 2 (node 2): {binary2_version}\n\n"
                f"Set expect_breaking=false in configs/binary-compat-package.nix"
            )


if __name__ == "__main__":
    import sys
    pytest.main([__file__, "-v", "-s"] + sys.argv[1:])
