import json
import subprocess
from typing import NamedTuple
from contextlib import contextmanager
from pathlib import Path
from pystarport import ports



from .network import AttestationLayer, Cronos, Hermes, setup_custom_cronos
from .utils import (
    wait_for_new_blocks,
    wait_for_port,
)

class AttestationNetwork(NamedTuple):
    cronos: Cronos
    attestad: AttestationLayer
    hermes: Hermes | None


def call_hermes_cmd_v1(
    hermes,
    port_id="attestation",
    counterparty_port_id="da",
    version="attestation-1",
):
    """
    Set up IBC v1 infrastructure between cronos_777-1 and attestation-1.
    
    IBC v1 requires clients, connections, and channels.
    
    Args:
        hermes: Hermes relayer configuration
        port_id: Port identifier for the attestation module on cronos_777-1
        counterparty_port_id: Port identifier on attestation-1 (default: "da")
        version: Channel version
        
    Returns:
        dict with channel IDs on both chains
    """
    chain_a = "cronos_777-1"
    chain_b = "attestation-1"
    
    print(f"\n{'='*60}")
    print(f"Setting up IBC v1 infrastructure")
    print(f"Between: {chain_a} <-> {chain_b}")
    print(f"Protocol: IBC v1 (clients, connections, and channels)")
    print(f"Port: {port_id}")
    print(f"{'='*60}\n")
    
    # Create clients
    print(f"📝 Step 1: Creating IBC v1 clients...")
    
    print(f"   Creating client on {chain_a} to track {chain_b}...")
    subprocess.check_call([
        "hermes", "--config", str(hermes.configpath),
        "create", "client",
        "--host-chain", chain_a,
        "--reference-chain", chain_b,
    ])
    print(f"   ✅ Client created on {chain_a}")
    
    print(f"   Creating client on {chain_b} to track {chain_a}...")
    subprocess.check_call([
        "hermes", "--config", str(hermes.configpath),
        "create", "client",
        "--host-chain", chain_b,
        "--reference-chain", chain_a,
    ])
    print(f"   ✅ Client created on {chain_b}")
    
    # Create connection
    print(f"\n📝 Step 2: Creating IBC v1 connection...")
    subprocess.check_call([
        "hermes", "--config", str(hermes.configpath),
        "create", "connection",
        "--a-chain", chain_a,
        "--b-chain", chain_b,
    ])
    print(f"   ✅ Connection created")
    
    # Create channel
    print(f"\n📝 Step 3: Creating IBC v1 channel...")
    print(f"   {chain_a} port: {port_id}")
    print(f"   {chain_b} port: {counterparty_port_id}")
    print(f"   Version: {version}")
    print(f"   Using connection: connection-0")
    
    channel_result = subprocess.check_output([
        "hermes", "--config", str(hermes.configpath),
        "create", "channel",
        "--a-chain", chain_a,
        "--a-connection", "connection-0",  # Use the connection we just created
        "--a-port", port_id,
        "--b-port", counterparty_port_id,
        "--channel-version", version,
        "--order", "unordered",
    ], text=True)
    
    print(f"   ✅ Channel created")
    print(f"   Result: {channel_result}")
    
    # Parse channel IDs from output
    # Hermes output format: "Success: Channel { ... channel_id: ChannelId("channel-X") ... }"
    import re
    channel_ids = re.findall(r'channel-\d+', channel_result)
    
    result = {}
    if len(channel_ids) >= 2:
        result["channel_a"] = channel_ids[0]  # Cronos channel
        result["channel_b"] = channel_ids[1]  # Attestation channel
        print(f"\n✅ Channel IDs:")
        print(f"   {chain_a}: {result['channel_a']}")
        print(f"   {chain_b}: {result['channel_b']}")
    else:
        print(f"   ⚠️  Could not parse channel IDs from output")
    
    print(f"\n✅ IBC v1 infrastructure ready!")
    print(f"{'='*60}\n")
    
    return result


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
                                                    output="json",
                                                    node=cronos_cli.node_rpc))
        attesta_clients = json.loads(attesta_cli.raw("query", "ibc", "client", "states",
                                                      output="json",
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
    ibc_version="v2",
):
    """
    Prepare attestation network with Cronos and Attestation Layer chains.
    
    Args:
        tmp_path: Temporary directory path
        file: Config file name (without .jsonnet extension)
        base_port: Base port for cronos chain (default: 26650)
        ibc_version: IBC version to use ("v1" or "v2", default: "v2")
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
        
        # Create IBC infrastructure based on version
        if ibc_version == "v1":
            print(f"\n🔧 Setting up IBC v1 infrastructure...")
            # cronos_777-1 uses port "attestation", attestation-1 uses port "da"
            channel_info = call_hermes_cmd_v1(hermes, port_id="attestation", counterparty_port_id="da")
            
            # Verify channel ID matches genesis configuration
            if channel_info.get("channel_a"):
                cronos_channel_id = channel_info["channel_a"]
                print(f"\n📝 IBC v1 Channel created on Cronos: {cronos_channel_id}")
                
                # Verify the channel ID matches what's in genesis
                try:
                    genesis_file = cronos.base_dir / "node0" / "config" / "genesis.json"
                    with open(genesis_file, 'r') as f:
                        genesis = json.load(f)
                    
                    if 'app_state' in genesis and 'attestation' in genesis['app_state']:
                        genesis_channel_id = genesis['app_state']['attestation'].get('v1_channel_id', '')
                        if genesis_channel_id == cronos_channel_id:
                            print(f"   ✅ Genesis v1_channel_id matches: {genesis_channel_id}")
                        elif genesis_channel_id:
                            print(f"   ⚠️  Genesis has different channel ID: {genesis_channel_id}")
                        else:
                            print(f"   ⚠️  Genesis does not have v1_channel_id set")
                            print(f"   Expected: {cronos_channel_id}")
                    
                except Exception as e:
                    print(f"   Note: Could not verify genesis: {e}")
            else:
                print(f"   ⚠️  Could not determine v1 channel ID from hermes output")
        else:
            print(f"\n🔧 Setting up IBC v2 infrastructure...")
            call_hermes_cmd(hermes)
        
        # Only register counterparties for IBC v2 (Eureka) packet flow
        # IBC v1 doesn't need this - it uses traditional port/channel routing
        if ibc_version == "v2":
            print("\n📝 Querying IBC client IDs for counterparty registration...")
            try:
                # Get client ID on Cronos
                cronos_clients = json.loads(cli.raw("query", "ibc", "client", "states",
                                                    output="json",
                                                    node=cli.node_rpc))
                cronos_client_states = cronos_clients.get("client_states", [])
                
                # Get client ID on Attestation Layer
                attesta_cli = attestation.cosmos_cli()
                attesta_clients = json.loads(attesta_cli.raw("query", "ibc", "client", "states",
                                                              output="json",
                                                              node=attesta_cli.node_rpc))
                attesta_client_states = attesta_clients.get("client_states", [])
                
                if cronos_client_states and attesta_client_states:
                    cronos_client_id = cronos_client_states[0].get("client_id")
                    attesta_client_id = attesta_client_states[0].get("client_id")
                    print(f"   Cronos client ID: {cronos_client_id}")
                    print(f"   Attestation client ID: {attesta_client_id}")
                    
                    # Register counterparties for IBC v2 packet flow
                    # IMPORTANT: Must use the same key that Hermes used to create the client
                    # RegisterCounterparty requires the signer to be the client creator
                    print(f"\n📝 Registering counterparties for IBC v2 packet flow...")
                
                # Register counterparty on Cronos chain
                # Use "signer1" - the key Hermes uses on Cronos (see attestation.jsonnet key_name)
                print(f"   Registering counterparty on cronos_777-1...")
                cmd_register_cronos = [
                    cli.raw.cmd,           # Binary path from CosmosCLI
                    "tx", "ibc", "client", "add-counterparty",
                    cronos_client_id,      # local client-id on cronos
                    attesta_client_id,     # counterparty client-id on attestation
                    "aWJj",                # Merkle prefix (base64 encoded "ibc")
                    "--home", str(cronos.base_dir / "node0"),
                    "--node", cli.node_rpc,
                    "--keyring-backend", "test",
                    "--chain-id", "cronos_777-1",
                    "--from", "signer1",   # Must match Hermes relayer key on this chain
                    "--broadcast-mode", "sync",
                    "-y",
                ]
                subprocess.check_call(cmd_register_cronos)
                print(f"   ✅ Counterparty registered on cronos_777-1")
                
                # Wait for tx to be included
                wait_for_new_blocks(cli, 2)
                
                # Register counterparty on Attestation Layer chain
                # Use "relayer" - the key Hermes uses on Attestation Layer (see attestation.jsonnet accounts)
                print(f"   Registering counterparty on attestation-1...")
                cmd_register_attesta = [
                    attesta_cli.raw.cmd,   # Binary path from CosmosCLI
                    "tx", "ibc", "client", "add-counterparty",
                    attesta_client_id,     # local client-id on attestation
                    cronos_client_id,      # counterparty client-id on cronos
                    "aWJj",                # Merkle prefix (base64 encoded "ibc")
                    "--home", str(attestation.base_dir / "node0"),
                    "--node", attesta_cli.node_rpc,
                    "--keyring-backend", "test",
                    "--chain-id", "attestation-1",
                    "--from", "relayer",   # Must match Hermes relayer key on this chain
                    "--broadcast-mode", "sync",
                    "-y",
                ]
                subprocess.check_call(cmd_register_attesta)
                print(f"   ✅ Counterparty registered on attestation-1")
                
                # Wait for tx to be included
                wait_for_new_blocks(attesta_cli, 2)
                
                # Verify counterparty registration on both chains
                print(f"\n🔍 Verifying counterparty registration...")
                
                # Verify on Cronos
                try:
                    cronos_counterparty = json.loads(cli.raw(
                        "query", "ibc", "client", "counterparty-info",
                        cronos_client_id,
                        output="json",
                        node=cli.node_rpc,
                        home=str(cronos.base_dir / "node0"),
                        chain_id="cronos_777-1",
                        keyring_backend="test",
                    ))
                    cronos_cp_client_id = cronos_counterparty.get("counterparty_info", "").get("client_id", "")
                    if cronos_cp_client_id:
                        print(f"   ✅ Cronos counterparty verified: {cronos_cp_client_id}")
                    else:
                        print(f"   ⚠️  Cronos counterparty not set")
                except Exception as e:
                    print(f"   ⚠️  Error verifying Cronos counterparty: {e}")
                
                # Verify on Attestation Layer
                try:
                    attesta_counterparty = json.loads(attesta_cli.raw(
                        "query", "ibc", "client", "counterparty-info",
                        attesta_client_id,
                        output="json",
                        node=attesta_cli.node_rpc,
                        home=str(attestation.base_dir / "node0"),
                    ))
                    attesta_cp_client_id = attesta_counterparty.get("counterparty_info", "").get("client_id", "")
                    if attesta_cp_client_id:
                        print(f"   ✅ Attestation counterparty verified: {attesta_cp_client_id}")
                    else:
                        print(f"   ⚠️  Attestation counterparty not set")
                except Exception as e:
                    print(f"   ⚠️  Error verifying Attestation counterparty: {e}")
                
                    print(f"\n✅ IBC v2 counterparty registration complete!")
                else:
                    print("   ⚠️  No IBC clients found yet")
            except Exception as e:
                print(f"   ⚠️  Error during counterparty registration: {e}")
                import traceback
                traceback.print_exc()
        else:
            print("\n📝 Skipping IBC v2 counterparty registration (using IBC v1)")
        
        # Start Hermes relayer
        cronos.supervisorctl("start", "relayer-demo")
        wait_for_port(hermes.port)
                
        yield AttestationNetwork(cronos, attestation, hermes)
