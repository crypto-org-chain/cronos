"""RPC-only devnet preflight checks.

Sysctl tuning and the libp2p-transport-enabled log line need host access,
which this tool doesn't have — it only ever talks to nodes over RPC. This
module covers what RPC *can* see: the mempool type each node declared, and
whether every node's peer set actually forms a connected mesh.
"""

from .utils import net_info, node_id


def resolved_mempool_types(endpoints):
    """{endpoint.name: declared mempool.type}, or None if undeclared."""
    return {
        endpoint.name: (getattr(endpoint, "node_config", None) or {}).get("mempool.type")
        for endpoint in endpoints
    }


def probe_peers(endpoints):
    """(node_ids, peer_ids) keyed by endpoint name: each node's own CometBFT node
    ID from `/status`, and the set of peer IDs from its `/net_info`. Either is
    None when that call failed, which is what marks the node unreachable."""
    node_ids = {}
    peer_ids = {}
    for endpoint in endpoints:
        try:
            node_ids[endpoint.name] = node_id(endpoint.rpc)
        except Exception:
            node_ids[endpoint.name] = None
        try:
            info = net_info(endpoint.rpc)["result"]
            peer_ids[endpoint.name] = {
                peer["node_info"]["id"] for peer in info.get("peers", [])
            }
        except Exception:
            peer_ids[endpoint.name] = None
    return node_ids, peer_ids


def unreachable_nodes(node_ids, peer_ids):
    """Names of nodes whose `/status` or `/net_info` didn't answer.

    Read from the probe rather than inferred from an all-None matrix row: with
    two endpoints the live node's row is all-None too (its only peer is
    unidentifiable), so the row shape can't tell which node is actually down.
    """
    return [
        name
        for name in node_ids
        if node_ids[name] is None or peer_ids.get(name) is None
    ]


def peer_connectivity_matrix(endpoints, probe=None):
    """{endpoint.name: {other.name: bool}} — whether `other` shows up in
    `endpoint`'s `/net_info` peer list, matched by CometBFT node ID.

    Node IDs come from each endpoint's `/status`. Matching on the RPC host
    instead would be meaningless for the common topologies: every node of a
    local devnet answers on 127.0.0.1, so every pair looks connected no matter
    what the real peer set is, and a node reachable over a DNS name or a
    separate RPC interface never matches its own p2p `remote_ip`.

    An endpoint whose `/status` or `/net_info` call fails gets None for every
    entry in its row instead of raising, so one unreachable node doesn't hide
    the rest of the matrix. A peer the endpoint knows but has no live
    connection to is not listed by `/net_info`, so it reports False.

    `probe` reuses a probe_peers result instead of re-querying every node.
    """
    node_ids, peer_ids = probe if probe is not None else probe_peers(endpoints)

    matrix = {}
    for endpoint in endpoints:
        ids = peer_ids[endpoint.name]
        row = {}
        for other in endpoints:
            if other.name == endpoint.name:
                continue
            other_id = node_ids[other.name]
            # An unidentifiable peer is unknown, not disconnected.
            row[other.name] = (
                None if ids is None or other_id is None else other_id in ids
            )
        matrix[endpoint.name] = row
    return matrix
