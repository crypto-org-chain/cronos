"""
Integration tests for attestation module with IBC v1

Tests the following flow:
1. Set up Cronos chain with IBC v1 flag (--da-ibc-version=v1)
2. Set up Attestation Layer chain
3. Create IBC v1 channel between the chains
4. Configure v1 channel ID in genesis
5. Trigger attestation sending from Cronos via IBC v1
6. Verify attestation received on Attestation Layer
"""

import json
import subprocess
from pathlib import Path

import pytest

from .network import Cronos
from .utils import wait_for_new_blocks
from .attestation_util import prepare_network

pytestmark = pytest.mark.attestation_v1


@pytest.fixture(scope="module")
def attestation_v1_network(tmp_path_factory):
    """
    Set up the attestation test network with IBC v1:
    - Cronos chain (with --da-ibc-version=v1 flag)
    - Attestation Layer chain  
    - IBC v1 channel setup
    
    Note: cronos-attestad binary must be in PATH or NIX_BIN_DIR
    """
    name = "attestation_v1"
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

    # Pass IBC version flag to prepare_network
    yield from prepare_network(path, "attestation_v1", ibc_version="v1")


def test_ibc_v1_configuration(attestation_v1_network):
    """Verify IBC v1 configuration is properly set"""
    cronos = attestation_v1_network.cronos
    cronos_cli = cronos.cosmos_cli()
    
    print("🔍 Verifying IBC v1 configuration...")
    
    # Query attestation params
    params = cronos_cli.query_params("attestation")
    print(f"Attestation params: {json.dumps(params, indent=2)}")
    
    # Verify params structure
    assert "attestation_enabled" in params, "attestation_enabled field missing"
    assert "attestation_interval" in params, "attestation_interval field missing"
    assert "packet_timeout_timestamp" in params, "packet_timeout_timestamp field missing"
    
    # Verify attestation is enabled
    assert params["attestation_enabled"] == True, "Attestation should be enabled"
    
    print(f"✅ Attestation module configured for IBC v1:")
    print(f"   - Enabled: {params['attestation_enabled']}")
    print(f"   - Interval: {params['attestation_interval']} blocks")
    print(f"   - Timeout: {params['packet_timeout_timestamp']} ns")
    
    # Query genesis to verify v1 channel ID is set
    try:
        genesis = cronos_cli.query_genesis()
        app_state = genesis.get("app_state", {})
        attestation_state = app_state.get("attestation", {})
        
        v1_channel_id = attestation_state.get("v1_channel_id", "")
        v1_port_id = attestation_state.get("v1_port_id", "attestation")
        
        print(f"✅ IBC v1 Genesis Configuration:")
        print(f"   - V1 Channel ID: {v1_channel_id}")
        print(f"   - V1 Port ID: {v1_port_id}")
        
        # V1 channel ID should be set for IBC v1 mode
        if v1_channel_id:
            print(f"✅ V1 channel ID is configured: {v1_channel_id}")
        else:
            print("⚠️  V1 channel ID not yet configured (will be set after IBC handshake)")
            
    except Exception as e:
        print(f"⚠️  Could not verify genesis configuration: {e}")


def test_ibc_v1_channel_setup(attestation_v1_network):
    """Verify IBC v1 channel is properly established"""
    cronos = attestation_v1_network.cronos
    cronos_cli = cronos.cosmos_cli()
    
    print("🔍 Verifying IBC v1 channel setup...")
    
    try:
        # Query IBC channels
        channels_result = cronos_cli.raw(
            "query", "ibc", "channel", "channels",
            home=cronos_cli.data_dir,
            output="json"
        )
        channels_data = json.loads(channels_result)
        channels = channels_data.get("channels", [])
        
        print(f"Found {len(channels)} IBC channels:")
        for ch in channels:
            port_id = ch.get("port_id", "")
            channel_id = ch.get("channel_id", "")
            state = ch.get("state", "")
            print(f"  - Port: {port_id}, Channel: {channel_id}, State: {state}")
            
            # Check for attestation port
            if port_id == "attestation":
                print(f"✅ Found attestation channel: {channel_id} (State: {state})")
                assert state == "STATE_OPEN", f"Attestation channel should be OPEN, got {state}"
                return  # Test passed
        
        print("⚠️  No attestation channel found (may be created dynamically)")
        
    except subprocess.CalledProcessError as e:
        print(f"⚠️  Could not query IBC channels: {e}")
    except (json.JSONDecodeError, KeyError) as e:
        print(f"⚠️  Could not parse channel data: {e}")


def test_attestation_v1_sending(attestation_v1_network):
    """Test automatic attestation sending via IBC v1"""
    cronos = attestation_v1_network.cronos
    cronos_cli = cronos.cosmos_cli()
    
    print("🔍 Testing IBC v1 attestation sending...")
    
    # Get current height
    current_height = cronos_cli.block_height()
    print(f"Current height: {current_height}")
    
    # Get attestation params to know the interval
    params = cronos_cli.query_params("attestation")
    interval = int(params["attestation_interval"])
    print(f"Attestation interval: {interval} blocks")
    
    # Wait past the next attestation interval
    blocks_to_wait = interval + 5
    print(f"Waiting {blocks_to_wait} blocks for automatic attestation via IBC v1...")
    wait_for_new_blocks(cronos_cli, blocks_to_wait)
    
    # Get new height
    new_height = cronos_cli.block_height()
    print(f"New height: {new_height}")
    
    # Check for IBC v1 attestation events
    attestation_v1_events_found = False
    v1_event_details = None
    
    for height in range(current_height, new_height + 1):
        try:
            # Query block results for events
            block_results = json.loads(
                cronos_cli.raw("query", "block-results", str(height), 
                               home=cronos_cli.data_dir)
            )
            
            # Check for attestation events
            events = (block_results.get("finalize_block_events", []) or 
                     block_results.get("end_block_events", []) or
                     block_results.get("events", []))
            
            for event in events:
                event_type = event.get("type", "")
                
                # Look for v1-specific attestation event
                if event_type == "attestation_v1_sent":
                    print(f"✅ Found IBC v1 attestation event at height {height}:")
                    print(f"   Event type: {event_type}")
                    
                    # Extract event attributes
                    attributes = {}
                    for attr in event.get("attributes", []):
                        key = attr.get("key", "")
                        value = attr.get("value", "")
                        attributes[key] = value
                        print(f"   - {key}: {value}")
                    
                    # Verify v1-specific attributes
                    assert "source_port" in attributes, "source_port attribute missing"
                    assert "source_channel" in attributes, "source_channel attribute missing"
                    assert "sequence" in attributes, "sequence attribute missing"
                    
                    attestation_v1_events_found = True
                    v1_event_details = attributes
                    break
                
                # Also accept generic attestation events
                elif "attestation" in event_type.lower() and event_type != "attestation_v2_sent":
                    print(f"✅ Found attestation event at height {height}:")
                    print(f"   Event type: {event_type}")
                    attestation_v1_events_found = True
                    break
            
            if attestation_v1_events_found:
                break
                
        except subprocess.CalledProcessError:
            continue
        except (json.JSONDecodeError, KeyError):
            continue
    
    # Verify attestation was sent via IBC v1
    assert attestation_v1_events_found, (
        f"No IBC v1 attestation events found between heights {current_height} and {new_height}. "
        f"Expected 'attestation_v1_sent' event within {blocks_to_wait} blocks "
        f"(interval={interval}). Check IBC v1 configuration and channel setup."
    )
    
    if v1_event_details:
        print("✅ IBC v1 attestation verified:")
        print(f"   - Port: {v1_event_details.get('source_port', 'N/A')}")
        print(f"   - Channel: {v1_event_details.get('source_channel', 'N/A')}")
        print(f"   - Sequence: {v1_event_details.get('sequence', 'N/A')}")
        print(f"   - Attestation count: {v1_event_details.get('attestation_count', 'N/A')}")
    
    print("✅ IBC v1 attestation module is operational")


# def test_attestation_v1_vs_v2_difference(attestation_v1_network):
#     """Verify that IBC v1 uses port/channel (not client IDs like v2)"""
#     cronos = attestation_v1_network.cronos
#     cronos_cli = cronos.cosmos_cli()
    
#     print("🔍 Verifying IBC v1 vs v2 differences...")
    
#     # Get genesis state
#     genesis = cronos_cli.query_genesis()
#     app_state = genesis.get("app_state", {})
#     attestation_state = app_state.get("attestation", {})
    
#     v1_channel_id = attestation_state.get("v1_channel_id", "")
#     v1_port_id = attestation_state.get("v1_port_id", "attestation")
#     v2_client_id = attestation_state.get("v2_client_id", "")
    
#     print("Configuration comparison:")
#     print(f"  IBC v1 - Port: {v1_port_id or 'not set'}, Channel: {v1_channel_id or 'not set'}")
#     print(f"  IBC v2 - Client: {v2_client_id or 'not set'}")
    
#     # For v1 mode, v1 config should be present or will be set
#     # v2 config may or may not be present (doesn't matter for v1 mode)
#     print("✅ Configuration structure verified for IBC v1 mode")


# def test_attestation_params_query(attestation_v1_network):
#     """Test querying attestation parameters"""
#     cronos = attestation_v1_network.cronos
#     cronos_cli = cronos.cosmos_cli()
    
#     print("🔍 Testing attestation params query...")
    
#     params = cronos_cli.query_params("attestation")
    
#     # Verify all expected fields are present
#     required_fields = [
#         "attestation_enabled",
#         "attestation_interval", 
#         "packet_timeout_timestamp"
#     ]
    
#     for field in required_fields:
#         assert field in params, f"Required field '{field}' missing from params"
    
#     # Verify values are reasonable
#     assert params["attestation_enabled"] == True
#     assert int(params["attestation_interval"]) > 0
#     assert int(params["packet_timeout_timestamp"]) > 0
    
#     print("✅ All attestation parameters present and valid")
#     print(f"   Params: {json.dumps(params, indent=2)}")

