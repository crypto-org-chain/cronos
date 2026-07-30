import json

from remote_benchmark.libp2p import bootstrap_peers, libp2p_id_from_node_key


def _write_node_key(tmp_path, name, priv_b64):
    path = tmp_path / f"{name}-node_key.json"
    path.write_text(json.dumps({"priv_key": {"type": "tendermint/PrivKeyEd25519", "value": priv_b64}}))
    return path


def test_libp2p_id_from_node_key_is_deterministic_and_uses_identity_multihash(tmp_path):
    # 64-byte tendermint ed25519 priv (seed || pub); pub is the last 32 bytes.
    # The marshaled PublicKey is 36 bytes (<=42), so encoding uses the
    # identity multihash (code 0x00), whose leading zero byte base58-encodes
    # to a leading '1'.
    priv = bytes(range(32)) + bytes(range(32, 64))
    import base64

    path = _write_node_key(tmp_path, "n0", base64.b64encode(priv).decode())

    peer_id = libp2p_id_from_node_key(path)

    assert isinstance(peer_id, str)
    assert peer_id.startswith("1")
    assert libp2p_id_from_node_key(path) == peer_id


def test_libp2p_id_from_node_key_rejects_wrong_length_pubkey(tmp_path):
    path = _write_node_key(tmp_path, "bad", __import__("base64").b64encode(b"\x00" * 40).decode())

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
