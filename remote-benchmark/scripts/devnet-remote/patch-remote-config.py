#!/usr/bin/env python3
"""Rewrite a freshly `pystarport init`'d chain dir's per-node config.toml /
app.toml from loopback-only to the real multi-host remote devnet topology.

pystarport always binds every node to 127.0.0.1 and advertises 127.0.0.1 to
its peers, regardless of jsonnet `hostname` settings (confirmed by reading
its generated tasks.ini output) - fine for a single-machine devnet, useless
across 5 separate hosts. This patches each node's home dir in place:

  - [p2p] laddr / [rpc] laddr -> bind 0.0.0.0 so the process accepts
    connections from other hosts, not just localhost.
  - [p2p] external_address -> this node's own private IP, so its
    advertised address is dialable by the other 4 hosts.
  - persistent_peers -> each 127.0.0.1:<port> replaced by the private IP of
    whichever node owns that port (looked up by port, since pystarport's
    peer-ID ordering within persistent_peers is not guaranteed to match
    node index order).
  - [json-rpc] address / ws-address -> bind 0.0.0.0, for the load generator
    reaching in from off-VPC over the public IP.

api/grpc/pprof are deliberately left on loopback - nothing off-host needs
them, and leaving them bound locally keeps the public surface smaller.

Uses regex line rewrites rather than a TOML library on purpose: pystarport's
own tomlkit dependency has a documented reparenting bug (see
remote_benchmark/fix_p2p_config.py) that corrupts [p2p] when subsections are
merged in. Re-parsing and re-serializing with a different tomlkit version
risks re-triggering or masking that class of bug; staying at the text level
avoids it entirely.

NODE_PRIVATE_IPS is read from hosts.env (or --hosts-env), so a differently
sized cluster (e.g. hosts-15val.env) needs no change here.
"""

import re
import subprocess
import sys
from pathlib import Path

BASE_PORT = 26650


def node_private_ips(hosts_env: Path) -> list[str]:
    out = subprocess.run(
        ["bash", "-c", f'source "{hosts_env}" && printf "%s\\n" "${{NODE_PRIVATE_IPS[@]}}"'],
        capture_output=True,
        text=True,
        check=True,
    )
    return out.stdout.splitlines()


# Populated by main() from hosts.env / --hosts-env before any of the patch_*
# functions below run.
NODE_PRIVATE_IPS: list[str] = []



def p2p_port(i: int) -> int:
    return BASE_PORT + i * 10


def rpc_port(i: int) -> int:
    return BASE_PORT + i * 10 + 7


def jsonrpc_port(i: int) -> int:
    return BASE_PORT + i * 10 + 1


def ws_port(i: int) -> int:
    return BASE_PORT + i * 10 + 2


def _replace_one(text: str, pattern: str, replacement: str, path: Path) -> str:
    new_text, count = re.subn(pattern, replacement, text, count=1, flags=re.MULTILINE)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly 1 match for {pattern!r}, got {count}")
    return new_text


def patch_config_toml(path: Path, node_index: int) -> None:
    text = path.read_text()

    text = _replace_one(
        text,
        r'^laddr = "tcp://127\.0\.0\.1:\d+"(?=\n\s*(?:#.*\n\s*)*external_address)',
        f'laddr = "tcp://0.0.0.0:{p2p_port(node_index)}"',
        path,
    )
    text = _replace_one(
        text,
        r'^external_address = ""',
        f'external_address = "tcp://{NODE_PRIVATE_IPS[node_index]}:{p2p_port(node_index)}"',
        path,
    )
    text = _replace_one(
        text,
        r'^laddr = "tcp://127\.0\.0\.1:\d+"(?=\n\s*(?:#.*\n\s*)*cors_allowed_origins)',
        f'laddr = "tcp://0.0.0.0:{rpc_port(node_index)}"',
        path,
    )

    peers_match = re.search(r'^persistent_peers = "([^"]*)"', text, re.MULTILINE)
    if not peers_match:
        raise SystemExit(f"{path}: persistent_peers line not found")
    port_to_ip = {p2p_port(i): ip for i, ip in enumerate(NODE_PRIVATE_IPS)}
    new_entries = []
    for entry in peers_match.group(1).split(","):
        node_id, host = entry.split("@")
        host_ip, port = host.split(":")
        port = int(port)
        if port not in port_to_ip:
            raise SystemExit(f"{path}: persistent_peers port {port} not in known mapping")
        new_entries.append(f"{node_id}@{port_to_ip[port]}:{port}")
    text = text[: peers_match.start()] + f'persistent_peers = "{",".join(new_entries)}"' + text[peers_match.end() :]

    path.write_text(text)


def patch_app_toml(path: Path, node_index: int) -> None:
    text = path.read_text()
    text = _replace_one(
        text,
        r'^address = "127\.0\.0\.1:\d+"(?=\n\s*(?:#.*\n\s*)*ws-address)',
        f'address = "0.0.0.0:{jsonrpc_port(node_index)}"',
        path,
    )
    text = _replace_one(
        text,
        r'^ws-address = "127\.0\.0\.1:\d+"',
        f'ws-address = "0.0.0.0:{ws_port(node_index)}"',
        path,
    )
    path.write_text(text)


def patch_libp2p(chain_dir: Path) -> None:
    """Rewrite bootstrap_peers for nodes with p2p.libp2p enabled, using
    private IPs instead of 127.0.0.1. No-op if libp2p is disabled (v1.7.8
    legacy-mempool config)."""
    node_dirs = sorted(chain_dir.glob("node*"), key=lambda p: int(p.name[len("node") :]))

    def libp2p_enabled(text: str) -> bool:
        header = re.search(r"^\[p2p\.libp2p\]\s*$", text, re.MULTILINE)
        if not header:
            return False
        next_table = re.search(r"^\[", text[header.end() :], re.MULTILINE)
        block = text[header.end() :][: next_table.start() if next_table else None]
        return re.search(r"^enabled = true", block, re.MULTILINE) is not None

    enabled = any(libp2p_enabled((d / "config" / "config.toml").read_text()) for d in node_dirs)
    if not enabled:
        return

    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
    from remote_benchmark.libp2p import append_bootstrap_peers_toml, bootstrap_peers

    nodes = [
        {
            "name": d.name,
            "ip": NODE_PRIVATE_IPS[int(d.name[len("node") :])],
            "port": p2p_port(int(d.name[len("node") :])),
            "node_key_path": d / "config" / "node_key.json",
        }
        for d in node_dirs
    ]
    peers = bootstrap_peers(nodes)
    # Local pipeline step (remote_benchmark.libp2p) already wrote 127.0.0.1
    # bootstrap_peers entries during pystarport init - strip those before
    # appending the private-IP versions, since append_bootstrap_peers_toml
    # is additive and stale loopback entries make libp2p dial 127.0.0.1
    # forever instead of the real host.
    loopback_block = re.compile(
        r"\n\[\[p2p\.libp2p\.bootstrap_peers\]\]\n"
        r'host = "127\.0\.0\.1:\d+"\n'
        r'id = "[^"]+"\n'
        r"persistent = \w+\n"
        r"unconditional = \w+\n"
    )
    for node in nodes:
        config_path = chain_dir / node["name"] / "config" / "config.toml"
        config_path.write_text(loopback_block.sub("", config_path.read_text()))
        append_bootstrap_peers_toml(config_path, peers[node["name"]])


def assert_topology(node_dirs: list[Path]) -> None:
    """Verify each node ended up peered with exactly every other node - no
    stray/missing entries, and max_peers has room for the full mesh (a hard
    connection gate with no exemption for bootstrap/unconditional peers)."""
    expected_n = len(node_dirs) - 1
    for node_dir in node_dirs:
        text = (node_dir / "config" / "config.toml").read_text()

        peers_match = re.search(r'^persistent_peers = "([^"]*)"', text, re.MULTILINE)
        if peers_match:
            entries = [e for e in peers_match.group(1).split(",") if e]
            if len(entries) != expected_n:
                raise SystemExit(f"{node_dir.name}: persistent_peers has {len(entries)} entries, expected {expected_n}")

        bootstrap_hosts = re.findall(r'^\[\[p2p\.libp2p\.bootstrap_peers\]\]\nhost = "([^"]+)"', text, re.MULTILINE)
        if bootstrap_hosts:
            if len(bootstrap_hosts) != expected_n:
                raise SystemExit(f"{node_dir.name}: bootstrap_peers has {len(bootstrap_hosts)} entries, expected {expected_n}")
            if any(host.startswith("127.0.0.1") for host in bootstrap_hosts):
                raise SystemExit(f"{node_dir.name}: stale loopback entry left in bootstrap_peers")
            max_peers_match = re.search(r"^max_peers = (\d+)", text, re.MULTILINE)
            if max_peers_match and int(max_peers_match.group(1)) <= expected_n:
                raise SystemExit(
                    f"{node_dir.name}: max_peers={max_peers_match.group(1)} <= {expected_n} peers - no room for the full mesh"
                )


def main() -> None:
    global NODE_PRIVATE_IPS

    args = list(sys.argv[1:])
    hosts_env = Path(__file__).parent / "hosts.env"
    if args and args[0] == "--hosts-env":
        if len(args) < 2:
            raise SystemExit(f"usage: {sys.argv[0]} [--hosts-env <path>] <chain-dir>")
        hosts_env = Path(args[1])
        args = args[2:]
    if len(args) != 1:
        raise SystemExit(f"usage: {sys.argv[0]} [--hosts-env <path>] <chain-dir>")
    chain_dir = Path(args[0])

    NODE_PRIVATE_IPS = node_private_ips(hosts_env)

    node_dirs = sorted(chain_dir.glob("node*"), key=lambda p: int(p.name[len("node") :]))
    if len(node_dirs) != len(NODE_PRIVATE_IPS):
        raise SystemExit(f"{chain_dir}: found {len(node_dirs)} node dirs, expected {len(NODE_PRIVATE_IPS)}")

    for node_dir in node_dirs:
        i = int(node_dir.name[len("node") :])
        patch_config_toml(node_dir / "config" / "config.toml", i)
        patch_app_toml(node_dir / "config" / "app.toml", i)

    patch_libp2p(chain_dir)

    assert_topology(node_dirs)

    for node_dir in node_dirs:
        i = int(node_dir.name[len("node") :])
        print(
            f"{node_dir.name}: p2p=0.0.0.0:{p2p_port(i)} external={NODE_PRIVATE_IPS[i]}:{p2p_port(i)} "
            f"rpc=0.0.0.0:{rpc_port(i)} json-rpc=0.0.0.0:{jsonrpc_port(i)}"
        )


if __name__ == "__main__":
    main()
