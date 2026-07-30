"""RPC-only devnet preflight checks.

Sysctl tuning and the libp2p-transport-enabled log line need host access,
which this tool doesn't have — it only ever talks to nodes over RPC. This
module covers what RPC *can* see: the mempool type each node declared, and
whether every node's peer set actually forms a connected mesh.
"""

from urllib.parse import urlparse

from .utils import net_info


def resolved_mempool_types(endpoints):
    """{endpoint.name: declared mempool.type}, or None if undeclared."""
    return {
        endpoint.name: (getattr(endpoint, "node_config", None) or {}).get("mempool.type")
        for endpoint in endpoints
    }


def peer_connectivity_matrix(endpoints):
    """{endpoint.name: {other.name: bool}} — whether `other` shows up in
    `endpoint`'s `/net_info` peer list, matched by RPC hostname/IP.

    An endpoint whose `/net_info` call fails gets None for every entry in its
    row instead of raising, so one unreachable node doesn't hide the rest of
    the matrix.
    """
    peer_ips = {}
    for endpoint in endpoints:
        try:
            info = net_info(endpoint.rpc)["result"]
            peer_ips[endpoint.name] = {peer["remote_ip"] for peer in info.get("peers", [])}
        except Exception:
            peer_ips[endpoint.name] = None

    matrix = {}
    for endpoint in endpoints:
        ips = peer_ips[endpoint.name]
        row = {}
        for other in endpoints:
            if other.name == endpoint.name:
                continue
            row[other.name] = None if ips is None else urlparse(other.rpc).hostname in ips
        matrix[endpoint.name] = row
    return matrix
