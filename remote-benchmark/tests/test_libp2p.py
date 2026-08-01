import base64
import json

from remote_benchmark.libp2p import bootstrap_peers, libp2p_id_from_node_key


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
