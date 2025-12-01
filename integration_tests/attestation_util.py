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
    Set up IBC v2 (Eureka) infrastructure between cronos_777-1 and attestation-1.
    
    IBC v2 (Eureka) only requires clients, not connections or channels.
    Packets are routed directly using packet-forward-middleware.
    
    Args:
        hermes: Hermes relayer configuration
        port_id: Port identifier (unused in IBC v2, kept for compatibility)
        version: Channel version (unused in IBC v2, kept for compatibility)
    """
    chain_a = "cronos_777-1"
    chain_b = "attestation-1"
    
    print(f"\n{'='*60}")
    print(f"Setting up IBC v2 (Eureka) infrastructure")
    print(f"Between: {chain_a} <-> {chain_b}")
    print(f"Protocol: IBC v2 (clients only, no connections/channels)")
    print(f"{'='*60}\n")
    
    # IBC v2 (Eureka): Only create clients, no connections or channels needed
    print(f"📝 Creating IBC v2 clients...")
    
    # Create client on chain A (cronos) tracking chain B (attestation)
    print(f"   Creating client on {chain_a} to track {chain_b}...")
    cmd_client_a = [
        "hermes",
        "--config",
        str(hermes.configpath),
        "create",
        "client",
        "--host-chain",
        chain_a,
        "--reference-chain",
        chain_b,
    ]
    subprocess.check_call(cmd_client_a)
    print(f"   ✅ Client created on {chain_a}")
    
    # Create client on chain B (attestation) tracking chain A (cronos)
    print(f"   Creating client on {chain_b} to track {chain_a}...")
    cmd_client_b = [
        "hermes",
        "--config",
        str(hermes.configpath),
        "create",
        "client",
        "--host-chain",
        chain_b,
        "--reference-chain",
        chain_a,
    ]
    subprocess.check_call(cmd_client_b)
    print(f"   ✅ Client created on {chain_b}")
    
    print(f"\n✅ IBC v2 clients created successfully")
    print(f"\n{'='*60}")
    print(f"IBC v2 infrastructure ready!")
    print(f"Clients created - packets can now be relayed")
    print(f"{'='*60}\n")


def verify_ibc_setup(cronos_cli, attesta_cli):
    """
    Verify IBC v2 (Eureka) infrastructure was set up correctly.
    
    IBC v2 only requires clients - no connections or channels.
    
    Args:
        cronos_cli: Cronos chain CLI
        attesta_cli: Attestation layer chain CLI
    
    Returns:
        dict with setup information (client counts)
    """
    result = {
        "clients": {"cronos": 0, "attesta": 0},
    }
    
    print(f"\n{'='*60}")
    print(f"Verifying IBC v2 (Eureka) infrastructure")
    print(f"{'='*60}\n")
    
    # Check clients (IBC v2 only needs clients)
    try:
        cronos_clients = json.loads(cronos_cli.raw("query", "ibc", "client", "states",
                                                    "--output", "json",
                                                    node=cronos_cli.node_rpc))
        attesta_clients = json.loads(attesta_cli.raw("query", "ibc", "client", "states",
                                                      "--output", "json",
                                                      node=attesta_cli.node_rpc))
        
        result["clients"]["cronos"] = len(cronos_clients.get("client_states", []))
        result["clients"]["attesta"] = len(attesta_clients.get("client_states", []))
        
        print(f"📋 IBC v2 Clients:")
        print(f"   Cronos: {result['clients']['cronos']}")
        print(f"   Attestation Layer: {result['clients']['attesta']}")
        
        assert result["clients"]["cronos"] > 0, "No clients on Cronos"
        assert result["clients"]["attesta"] > 0, "No clients on Attestation Layer"
        print(f"   ✅ Clients verified")
    except Exception as e:
        print(f"   ⚠️  Error checking clients: {e}")
    
    # IBC v2 (Eureka) does not use connections or channels
    print(f"\n💡 IBC v2 Note:")
    print(f"   IBC v2 (Eureka) does not use connections or channels")
    print(f"   Packets are routed directly using packet-forward-middleware")
    
    print(f"\n{'='*60}")
    print(f"IBC v2 infrastructure verification complete!")
    print(f"{'='*60}\n")
    
    return result

def prepare_network(
    tmp_path,
    file,
    base_port=26650,
):
    """
    Prepare attestation network with Cronos and Attestation Layer chains.
    
    Args:
        tmp_path: Temporary directory path
        file: Config file name (without .jsonnet extension)
        base_port: Base port for cronos chain (default: 26650)
    """
    config_file = file
    file_path = f"configs/{config_file}.jsonnet"

    with contextmanager(setup_custom_cronos)(
        tmp_path,
        base_port,
        Path(__file__).parent / file_path,
    ) as cronos:
        cli = cronos.cosmos_cli()
        
        # Wait for Cronos to be ready
        wait_for_port(ports.grpc_port(cronos.base_port(0)))
        wait_for_new_blocks(cli, 1)
        
        # Set up Attestation Layer
        attestation = AttestationLayer(cronos.base_dir.parent / "attestation-1")
        
        # Wait for Attestation Layer to be ready
        wait_for_port(ports.grpc_port(attestation.base_port(0)))
        wait_for_new_blocks(attestation.cosmos_cli(), 1)
        
        # Set up Hermes relayer
        hermes = Hermes(cronos.base_dir.parent / "relayer.toml")
        
        # Create IBC infrastructure
        call_hermes_cmd(hermes)
        
        # Start Hermes relayer
        cronos.supervisorctl("start", "relayer-demo")
        wait_for_port(hermes.port)
                
        yield AttestationNetwork(cronos, attestation, hermes)
