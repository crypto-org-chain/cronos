"""
Integration tests for attestation module with IBC v2

Tests the following flow:
1. Set up Cronos chain and Attestation Layer chain
2. Create IBC v2 clients between the chains
3. Configure Hermes relayer
4. Trigger attestation sending from Cronos
5. Verify attestation received on Attestation Layer
6. Verify finality feedback received on Cronos
"""

import json
import subprocess
from pathlib import Path

import pytest

from .network import Cronos
from .utils import wait_for_new_blocks
from .attestation_util import prepare_network

pytestmark = pytest.mark.attestation_v2





@pytest.fixture(scope="module")
def attestation_network(tmp_path_factory):
    """
    Set up the attestation test network:
    - Cronos chain
    - Attestation Layer chain  
    - Hermes relayer
    
    Note: cronos-attestad binary must be in PATH or NIX_BIN_DIR
    """
    name = "attestation"
    path = tmp_path_factory.mktemp(name)
    
    # Check if cronos-attestad is available
    import shutil
    import os
    
    attestad_path = shutil.which("cronos-attestad")
    if not attestad_path:
        # Try NIX_BIN_DIR
        nix_bin = os.environ.get("NIX_BIN_DIR", "/Users/randy.ang/Documents/code/cronos-attestation-layer/build")
        attestad_candidate = Path(nix_bin) / "cronos-attestad"
        if attestad_candidate.exists():
            attestad_path = str(attestad_candidate)
            os.environ["PATH"] = f"{nix_bin}:{os.environ['PATH']}"
        else:
            pytest.skip(f"cronos-attestad not found. Set NIX_BIN_DIR or add to PATH. Tried: {attestad_candidate}")
    
    print(f"Using cronos-attestad: {attestad_path}")

    yield from prepare_network(path, "attestation")


def test_attestation_module_enabled(attestation_network):
    """Verify attestation module is enabled and configured"""
    cronos = attestation_network.cronos
    cronos_cli = cronos.cosmos_cli()
    
    print("🔍 Querying attestation params...")
    
    # Query attestation params
    params = cronos_cli.query_params("attestation")
    print(f"Attestation params: {json.dumps(params, indent=2)}")
    
    # Verify params structure and values
    assert "attestation_enabled" in params, "attestation_enabled field missing"
    assert "attestation_interval" in params, "attestation_interval field missing"
    assert "packet_timeout_timestamp" in params, "packet_timeout_timestamp field missing"
    
    # Verify attestation is enabled
    assert params["attestation_enabled"] == True, "Attestation should be enabled"
    
    # Verify interval is positive
    interval = int(params["attestation_interval"])
    assert interval > 0, f"Attestation interval should be > 0, got {interval}"
    
    # Verify timeout is set
    timeout = int(params["packet_timeout_timestamp"])
    assert timeout > 0, f"Packet timeout should be > 0, got {timeout}"
    
    print(f"✅ Attestation module properly configured:")
    print(f"   - Enabled: {params['attestation_enabled']}")
    print(f"   - Interval: {params['attestation_interval']} blocks")
    print(f"   - Timeout: {params['packet_timeout_timestamp']} ns")


def test_send_attestation_manual(attestation_network):
    """Wait for automatic attestation send and verify it works"""
    cronos = attestation_network.cronos
    cronos_cli = cronos.cosmos_cli()
    
    print("🔍 Testing automatic attestation send...")
    
    # Get current height
    current_height = cronos_cli.block_height()
    print(f"Current height: {current_height}")
    
    # Get attestation params to know the interval
    params = cronos_cli.query_params("attestation")
    interval = int(params["attestation_interval"])
    print(f"Attestation interval: {interval} blocks")
    
    # Wait past the next attestation interval
    # Add a few extra blocks for processing
    blocks_to_wait = interval + 5
    print(f"Waiting {blocks_to_wait} blocks for automatic attestation...")
    wait_for_new_blocks(cronos_cli, blocks_to_wait)
    
    # Get new height
    new_height = cronos_cli.block_height()
    print(f"New height: {new_height}")
    
    # Check for attestation events in block events (simpler and more reliable)
    # Instead of querying transactions, check block events directly
    try:
        # Get a few recent blocks and check their events
        attestation_events_found = False
        for height in range(current_height, new_height + 1):
            try:
                # Query block results for events
                block_results = json.loads(
                    cronos_cli.raw("query", "block-results", str(height), 
                                   home=cronos_cli.data_dir)
                )
                
                # Check for attestation events in finalize_block_events or end_block_events
                events = (block_results.get("finalize_block_events", []) or 
                         block_results.get("end_block_events", []) or
                         block_results.get("events", []))
                
                for event in events:
                    event_type = event.get("type", "")
                    if "attestation" in event_type.lower():
                        print(f"✅ Found attestation event at height {height}:")
                        print(f"   Event type: {event_type}")
                        attestation_events_found = True
                        break
                
                if attestation_events_found:
                    break
                    
            except subprocess.CalledProcessError:
                # Block might not exist yet or query failed
                continue
            except (json.JSONDecodeError, KeyError):
                # Invalid response format
                continue
        
        # Attestation events must be found for the test to pass
        assert attestation_events_found, (
            f"No attestation events found between heights {current_height} and {new_height}. "
            f"Expected attestation to be sent within {blocks_to_wait} blocks "
            f"(interval={interval}). Check attestation module configuration and IBC v2 setup."
        )
        print("✅ Attestation module is operational - events detected")
        
    except AssertionError:
        # Re-raise assertion errors (test failures)
        raise
    except Exception as e:
        # Other errors should fail the test too
        pytest.fail(f"Failed to query attestation events: {e}")
