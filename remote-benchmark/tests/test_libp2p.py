import base64
import json

from remote_benchmark.libp2p import (
    append_bootstrap_peers_toml,
    bootstrap_peers,
    libp2p_id_from_node_key,
)


def _write_node_key(tmp_path, name, priv_b64):
    path = tmp_path / f"{name}-node_key.json"
    path.write_text(json.dumps({"priv_key": {"type": "tendermint/PrivKeyEd25519", "value": priv_b64}}))
    return path


def test_libp2p_id_from_node_key_matches_go_libp2p_for_a_known_key(tmp_path):
    # Golden vector: peer.IDFromPublicKey(crypto.UnmarshalEd25519PublicKey(pub))
    # from go-libp2p v0.48.0 (the version CometBFT pulls in) for the pubkey
    # bytes(range(32, 64)). Without a fixed expected string, a wrong protobuf
    # wrapper or multihash prefix still produces a deterministic, plausible ID.
    priv = bytes(range(32)) + bytes(range(32, 64))  # tendermint priv: seed || pub
    path = _write_node_key(tmp_path, "n0", base64.b64encode(priv).decode())

    peer_id = libp2p_id_from_node_key(path)

    assert peer_id == "12D3KooWBynX2HaNg73xSLq9TJDQjQKozxCh7MgVqKgGGXWXYzQn"
    assert libp2p_id_from_node_key(path) == peer_id


def test_libp2p_id_from_node_key_rejects_wrong_length_pubkey(tmp_path):
    path = _write_node_key(tmp_path, "bad", base64.b64encode(b"\x00" * 40).decode())

    try:
        libp2p_id_from_node_key(path)
        assert False, "expected ValueError"
    except ValueError:
        pass


def test_bootstrap_peers_excludes_self_and_includes_others(tmp_path, monkeypatch):
    import remote_benchmark.libp2p as libp2p_module

    monkeypatch.setattr(
        libp2p_module,
        "libp2p_id_from_node_key",
        lambda path: f"id-{path}",
    )

    nodes = [
        {"name": "n0", "ip": "10.0.0.1", "node_key_path": "n0-key"},
        {"name": "n1", "ip": "10.0.0.2", "node_key_path": "n1-key"},
        {"name": "n2", "ip": "10.0.0.3", "node_key_path": "n2-key"},
    ]

    result = bootstrap_peers(nodes, port=26656)

    assert result["n0"] == [
        {"host": "10.0.0.2:26656", "id": "id-n1-key", "persistent": True, "unconditional": True},
        {"host": "10.0.0.3:26656", "id": "id-n2-key", "persistent": True, "unconditional": True},
    ]
    assert len(result["n1"]) == 2
    assert all(entry["host"] != "10.0.0.2:26656" for entry in result["n1"])


def test_bootstrap_peers_uses_per_node_port_over_shared_default(tmp_path, monkeypatch):
    import remote_benchmark.libp2p as libp2p_module

    monkeypatch.setattr(libp2p_module, "libp2p_id_from_node_key", lambda path: f"id-{path}")

    nodes = [
        {"name": "n0", "ip": "127.0.0.1", "port": 26650, "node_key_path": "n0-key"},
        {"name": "n1", "ip": "127.0.0.1", "port": 26660, "node_key_path": "n1-key"},
    ]

    result = bootstrap_peers(nodes)

    assert result["n0"] == [
        {"host": "127.0.0.1:26660", "id": "id-n1-key", "persistent": True, "unconditional": True}
    ]
    assert result["n1"] == [
        {"host": "127.0.0.1:26650", "id": "id-n0-key", "persistent": True, "unconditional": True}
    ]


def test_append_bootstrap_peers_toml_writes_array_of_tables(tmp_path):
    config_toml = tmp_path / "config.toml"
    config_toml.write_text("[p2p]\nlisten_addr = \"tcp://0.0.0.0:26656\"\n")

    append_bootstrap_peers_toml(
        config_toml,
        [
            {"host": "127.0.0.1:26660", "id": "peer1", "persistent": True, "unconditional": True},
        ],
    )

    contents = config_toml.read_text()
    assert '[[p2p.libp2p.bootstrap_peers]]' in contents
    assert 'host = "127.0.0.1:26660"' in contents
    assert 'id = "peer1"' in contents
    assert 'persistent = true' in contents
    assert 'unconditional = true' in contents


def test_append_bootstrap_peers_toml_noop_on_empty_peers(tmp_path):
    config_toml = tmp_path / "config.toml"
    original = "[p2p]\nlisten_addr = \"tcp://0.0.0.0:26656\"\n"
    config_toml.write_text(original)

    append_bootstrap_peers_toml(config_toml, [])

    assert config_toml.read_text() == original
