import json
import subprocess
from typing import NamedTuple
from contextlib import contextmanager
from pathlib import Path
from pystarport import ports



from .network import AttestationLayer, Cronos, Hermes, setup_custom_cronos
from .utils import (
    CONTRACTS,
    deploy_contract,
    derive_new_account,
    send_transaction,
    wait_for_new_blocks,
    wait_for_port,
)

class AttestationNetwork(NamedTuple):
    cronos: Cronos
    attestad: AttestationLayer
    hermes: Hermes | None


def call_hermes_cmd(
    hermes,
    port_id="transfer",
    version=None,
):
    """
    Set up complete IBC infrastructure between cronos_777-1 and attestation-1.
    
    Following Celestia IBC relayer guide:
    https://docs.celestia.org/how-to-guides/ibc-relayer
    
    Creates clients, connection, and channel in one command using:
    hermes create channel --new-client-connection
    
    Args:
        hermes: Hermes relayer configuration
        port_id: Port identifier for the channel (default: "transfer")
        version: Channel version (optional, string or dict)
    """
    chain_a = "cronos_777-1"
    chain_b = "attestation-1"
    
    print(f"\n{'='*60}")
    print(f"Setting up IBC infrastructure (full)")
    print(f"Between: {chain_a} <-> {chain_b}")
    print(f"Port: {port_id}")
    print(f"{'='*60}\n")
    
    # Create channel with full setup (clients + connection + channel)
    # Following Celestia guide: hermes create channel with --new-client-connection
    print(f"📝 Creating full IBC setup (clients + connection + channel)...")
    
    cmd = [
        "hermes",
        "--config",
        str(hermes.configpath),
        "create",
        "channel",
        "--a-chain",
        chain_a,
        "--b-chain",
        chain_b,
        "--a-port",
        port_id,
        "--b-port",
        port_id,
        "--new-client-connection",
        "--yes",
    ]
    
    # Add channel version if specified
    if version:
        version_str = version if isinstance(version, str) else json.dumps(version)
        cmd.extend(["--channel-version", version_str])
        print(f"   Version: {version_str}")
    
    subprocess.check_call(cmd)
    print(f"✅ Full IBC setup complete")
    
    print(f"\n{'='*60}")
    print(f"IBC infrastructure created!")
    print(f"{'='*60}\n")


def verify_ibc_setup(cronos_cli, attesta_cli):
    """
    Verify complete IBC infrastructure was set up correctly.
    
    Checks for clients, connections, and channels on both chains.
    
    Args:
        cronos_cli: Cronos chain CLI
        attesta_cli: Attestation layer chain CLI
    
    Returns:
        dict with setup information (counts of clients, connections, channels)
    """
    result = {
        "clients": {"cronos": 0, "attesta": 0},
        "connections": {"cronos": 0, "attesta": 0},
        "channels": {"cronos": 0, "attesta": 0},
    }
    
    print(f"\n{'='*60}")
    print(f"Verifying IBC infrastructure")
    print(f"{'='*60}\n")
    
    # Check clients
    try:
        cronos_clients = cronos_cli.query("ibc client states")
        attesta_clients = attesta_cli.query("ibc client states")
        
        result["clients"]["cronos"] = len(cronos_clients.get("client_states", []))
        result["clients"]["attesta"] = len(attesta_clients.get("client_states", []))
        
        print(f"📋 IBC Clients:")
        print(f"   Cronos: {result['clients']['cronos']}")
        print(f"   Attestation Layer: {result['clients']['attesta']}")
        
        assert result["clients"]["cronos"] > 0, "No clients on Cronos"
        assert result["clients"]["attesta"] > 0, "No clients on Attestation Layer"
        print(f"   ✅ Clients verified")
    except Exception as e:
        print(f"   ⚠️  Error checking clients: {e}")
    
    # Check connections
    try:
        cronos_conns = cronos_cli.query("ibc connection connections")
        attesta_conns = attesta_cli.query("ibc connection connections")
        
        result["connections"]["cronos"] = len(cronos_conns.get("connections", []))
        result["connections"]["attesta"] = len(attesta_conns.get("connections", []))
        
        print(f"\n🔗 IBC Connections:")
        print(f"   Cronos: {result['connections']['cronos']}")
        print(f"   Attestation Layer: {result['connections']['attesta']}")
        
        assert result["connections"]["cronos"] > 0, "No connections on Cronos"
        assert result["connections"]["attesta"] > 0, "No connections on Attestation Layer"
        print(f"   ✅ Connections verified")
    except Exception as e:
        print(f"   ⚠️  Error checking connections: {e}")
    
    # Check channels
    try:
        cronos_channels = cronos_cli.query("ibc channel channels")
        attesta_channels = attesta_cli.query("ibc channel channels")
        
        result["channels"]["cronos"] = len(cronos_channels.get("channels", []))
        result["channels"]["attesta"] = len(attesta_channels.get("channels", []))
        
        print(f"\n📡 IBC Channels:")
        print(f"   Cronos: {result['channels']['cronos']}")
        print(f"   Attestation Layer: {result['channels']['attesta']}")
        
        assert result["channels"]["cronos"] > 0, "No channels on Cronos"
        assert result["channels"]["attesta"] > 0, "No channels on Attestation Layer"
        print(f"   ✅ Channels verified")
    except Exception as e:
        print(f"   ⚠️  Error checking channels: {e}")
    
    print(f"\n{'='*60}")
    print(f"IBC infrastructure verification complete!")
    print(f"{'='*60}\n")
    
    return result

def prepare_network(
    tmp_path,
    file,
):
    config_file = file
    file_path = f"configs/{config_file}.jsonnet"

    with contextmanager(setup_custom_cronos)(
        tmp_path,
        26700,
        Path(__file__).parent / file_path,
    ) as cronos:
        cli = cronos.cosmos_cli()
        path = cronos.base_dir.parent / "relayer"

        attestation = AttestationLayer(cronos.base_dir.parent / "attestation-1")
        # wait for grpc ready
        wait_for_port(ports.grpc_port(attestation.base_port(0)))  # attestation grpc
        wait_for_port(ports.grpc_port(attestation.base_port(0)))  # attestation grpc
        wait_for_new_blocks(attestation.cosmos_cli(), 1)
        wait_for_new_blocks(cli, 1)

        hermes = Hermes(path.with_suffix(".toml"))
        call_hermes_cmd(
            hermes,
        )

        cronos.supervisorctl("start", "attestation-relayer-demo")
        port = hermes.port
                
        yield AttestationNetwork(cronos, attestation, hermes)
        wait_for_port(port)
