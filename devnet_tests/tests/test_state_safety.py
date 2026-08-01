from itertools import cycle
from types import SimpleNamespace

from devnet_tests.devnet import Node
from devnet_tests.state_safety import (
    _http_url,
    _process_name,
    abci_app_hash,
    check_app_hash_agreement,
    historical_query_soak,
    open_fd_count,
)


def _node(name: str, scheme: str = "http") -> Node:
    return Node(name, w3=None, rpc=f"{scheme}://{name}:26657")


NODE_A, NODE_B, NODE_C = _node("a"), _node("b"), _node("c")
TCP_NODE = _node("t", scheme="tcp")


def _stub_abci_info(monkeypatch, responses):
    """responses: [(node, response)], where a response is the `/abci_info`
    result.response body or an Exception to raise. Keyed on the rewritten URL,
    matching what the probe actually requests. Returns the requested URLs."""
    by_url = {_http_url(node.rpc): response for node, response in responses}
    requested = []

    def get(url, timeout=None):
        requested.append(url)
        response = by_url[url.removesuffix("/abci_info")]
        if isinstance(response, Exception):
            raise response
        body = {"result": {"response": response}}
        return SimpleNamespace(json=lambda: body)

    monkeypatch.setattr("devnet_tests.state_safety.requests.get", get)
    return requested


def _committed(height, app_hash):
    return {"last_block_height": str(height), "last_block_app_hash": app_hash}


def _check(nodes, blocks=1):
    # interval/timeout of 0: every stub answers immediately, so one sampling pass
    # is all the loop can usefully do.
    return check_app_hash_agreement(nodes, blocks=blocks, interval=0, timeout=0)


def _reasons(divergences):
    return " | ".join(entry["reason"] for entry in divergences)


def test_abci_app_hash_reads_the_nodes_own_last_commit(monkeypatch):
    _stub_abci_info(monkeypatch, [(NODE_A, _committed(42, "beef"))])

    assert abci_app_hash(NODE_A) == (42, "beef")


def test_abci_app_hash_is_zero_before_anything_is_committed(monkeypatch):
    _stub_abci_info(monkeypatch, [(NODE_A, {})])

    assert abci_app_hash(NODE_A) == (0, "")


def test_abci_app_hash_requests_a_tcp_rpc_address_over_http(monkeypatch):
    requested = _stub_abci_info(monkeypatch, [(TCP_NODE, _committed(3, "same"))])

    abci_app_hash(TCP_NODE)

    assert requested == ["http://t:26657/abci_info"]


def test_check_app_hash_agreement_flags_a_disagreeing_height(monkeypatch):
    _stub_abci_info(
        monkeypatch,
        [(NODE_A, _committed(5, "hashA")), (NODE_B, _committed(5, "hashB"))],
    )

    divergences = _check([NODE_A, NODE_B])

    assert {
        "height": 5,
        "hashes": {"a": "hashA", "b": "hashB"},
        "reason": "mismatch",
    } in divergences


def test_check_app_hash_agreement_ignores_agreeing_heights(monkeypatch):
    _stub_abci_info(
        monkeypatch,
        [(NODE_A, _committed(5, "same")), (NODE_B, _committed(5, "same"))],
    )

    assert _check([NODE_A, NODE_B], blocks=0) == []


def test_check_app_hash_agreement_flags_a_node_that_never_answers(monkeypatch):
    # A diverged node panics on "wrong Block.Header.AppHash" and stops answering,
    # so dropping it from the comparison set would pass on a broken run.
    _stub_abci_info(
        monkeypatch,
        [
            (NODE_A, _committed(5, "hashA")),
            (NODE_B, _committed(5, "hashA")),
            (NODE_C, RuntimeError("connection refused")),
        ],
    )

    assert "no committed app hash from ['c']" in _reasons(
        _check([NODE_A, NODE_B, NODE_C], blocks=0)
    )


def test_check_app_hash_agreement_flags_a_chain_that_does_not_advance(monkeypatch):
    # A halted node must fail the check, not shrink the compared window.
    _stub_abci_info(
        monkeypatch,
        [(NODE_A, _committed(5, "same")), (NODE_B, _committed(5, "same"))],
    )

    assert "chain only advanced to 5" in _reasons(_check([NODE_A, NODE_B], blocks=2))


def test_check_app_hash_agreement_flags_a_node_reporting_two_hashes_at_a_height(
    monkeypatch,
):
    flapping = cycle(["first", "second"])
    monkeypatch.setattr(
        "devnet_tests.state_safety.abci_app_hash",
        lambda node: (5, "stable" if node is NODE_A else next(flapping)),
    )

    assert "b reported two app hashes for height 5" in _reasons(
        check_app_hash_agreement([NODE_A, NODE_B], blocks=9, interval=0, timeout=0.05)
    )


def test_check_app_hash_agreement_flags_an_absent_app_hash(monkeypatch):
    # last_block_app_hash is omitempty: if the key is absent or renamed, every
    # node reads back "" and they all "agree" having compared nothing.
    _stub_abci_info(
        monkeypatch,
        [
            (NODE_A, {"last_block_height": "5"}),
            (NODE_B, {"last_block_height": "5"}),
        ],
    )

    assert "empty app hash" in _reasons(_check([NODE_A, NODE_B], blocks=0))


def test_check_app_hash_agreement_does_not_call_an_empty_hash_node_silent(monkeypatch):
    # A node that answers with a committed height but no hash did respond, so
    # reporting it as silent too would double-report it and burn the full timeout
    # waiting for a tip it already gave us.
    _stub_abci_info(
        monkeypatch,
        [
            (NODE_A, _committed(5, "same")),
            (NODE_B, _committed(5, "same")),
            (NODE_C, {"last_block_height": "5"}),
        ],
    )

    reasons = _reasons(_check([NODE_A, NODE_B, NODE_C], blocks=0))

    assert "empty app hash" in reasons
    assert "no committed app hash" not in reasons


def test_check_app_hash_agreement_needs_two_nodes():
    assert "need two nodes" in _reasons(_check([NODE_A]))


def test_check_app_hash_agreement_flags_a_window_no_two_nodes_shared(monkeypatch):
    # Heights only ever reported by one node were never actually compared.
    _stub_abci_info(
        monkeypatch,
        [(NODE_A, _committed(5, "hashA")), (NODE_B, _committed(9, "hashB"))],
    )

    assert "no height was reported by two nodes" in _reasons(
        _check([NODE_A, NODE_B], blocks=0)
    )


def test_historical_query_soak_captures_errors_without_raising():
    calls = []

    def get_balance(address, height):
        calls.append((address, height))
        if len(calls) == 2:
            raise RuntimeError("too many open files")

    w3 = SimpleNamespace(eth=SimpleNamespace(get_balance=get_balance))

    result = historical_query_soak(w3, "0xabc", height=1, iterations=3)

    assert result.completed == 2
    assert result.errors == ["too many open files"]
    assert calls == [("0xabc", 1)] * 3


def test_historical_query_soak_reports_no_errors_on_a_clean_run():
    w3 = SimpleNamespace(eth=SimpleNamespace(get_balance=lambda address, height: 0))

    result = historical_query_soak(w3, "0xabc", height=1, iterations=5)

    assert (result.completed, result.errors) == (5, [])


def test_open_fd_count_is_unknown_for_a_remote_node():
    assert open_fd_count("tcp://node-1.example.com:26657") is None


def test_open_fd_count_is_unknown_when_nothing_is_listening(monkeypatch):
    monkeypatch.setattr("devnet_tests.state_safety._listening_pid", lambda port: None)
    assert open_fd_count("tcp://127.0.0.1:26657") is None


def test_open_fd_count_is_unknown_when_a_forwarder_holds_the_port(monkeypatch):
    monkeypatch.setattr("devnet_tests.state_safety._listening_pid", lambda port: 4242)
    monkeypatch.setattr(
        "devnet_tests.state_safety._process_name", lambda pid: "docker-proxy"
    )
    assert open_fd_count("http://127.0.0.1:8545") is None


def test_open_fd_count_counts_proc_fd_entries(monkeypatch):
    monkeypatch.setattr("devnet_tests.state_safety._listening_pid", lambda port: 4242)
    monkeypatch.setattr(
        "devnet_tests.state_safety._process_name", lambda pid: "cronosd"
    )
    monkeypatch.setattr(
        "devnet_tests.state_safety.os.path.isdir",
        lambda path: path == "/proc/4242/fd",
    )
    monkeypatch.setattr(
        "devnet_tests.state_safety.os.listdir", lambda path: ["0", "1", "2", "7"]
    )

    assert open_fd_count("http://127.0.0.1:8545") == 4


def test_open_fd_count_falls_back_to_lsof_without_proc(monkeypatch):
    monkeypatch.setattr("devnet_tests.state_safety._listening_pid", lambda port: 4242)
    monkeypatch.setattr(
        "devnet_tests.state_safety._process_name",
        lambda pid: "/opt/cronos/build/cronosd",
    )
    monkeypatch.setattr("devnet_tests.state_safety.os.path.isdir", lambda path: False)
    monkeypatch.setattr(
        "devnet_tests.state_safety.subprocess.run",
        lambda *a, **k: SimpleNamespace(returncode=0, stdout="HEADER\nfd1\nfd2\n"),
    )

    assert open_fd_count("http://127.0.0.1:8545") == 2


def test_open_fd_count_is_unknown_when_lsof_cannot_read_the_process(monkeypatch):
    # Header-only output (or a nonzero exit) means lsof couldn't read the fd
    # table at all - a live process always has fds beyond the header, so this
    # must read as unknown, never as a false zero.
    monkeypatch.setattr("devnet_tests.state_safety._listening_pid", lambda port: 4242)
    monkeypatch.setattr(
        "devnet_tests.state_safety._process_name",
        lambda pid: "/opt/cronos/build/cronosd",
    )
    monkeypatch.setattr("devnet_tests.state_safety.os.path.isdir", lambda path: False)
    monkeypatch.setattr(
        "devnet_tests.state_safety.subprocess.run",
        lambda *a, **k: SimpleNamespace(returncode=0, stdout="HEADER\n"),
    )

    assert open_fd_count("http://127.0.0.1:8545") is None

    monkeypatch.setattr(
        "devnet_tests.state_safety.subprocess.run",
        lambda *a, **k: SimpleNamespace(returncode=1, stdout=""),
    )

    assert open_fd_count("http://127.0.0.1:8545") is None


def test_open_fd_count_is_unknown_when_the_process_name_is_unreadable(monkeypatch):
    # An unverifiable name is not evidence the node holds the port, so the fd
    # delta must read as unknown rather than be counted for some forwarder.
    monkeypatch.setattr("devnet_tests.state_safety._listening_pid", lambda port: 4242)
    monkeypatch.setattr("devnet_tests.state_safety._process_name", lambda pid: None)

    assert open_fd_count("http://127.0.0.1:8545") is None


def test_process_name_falls_back_to_ps_without_proc(monkeypatch):
    monkeypatch.setattr(
        "devnet_tests.state_safety.Path.read_text",
        lambda self: (_ for _ in ()).throw(OSError("no /proc")),
    )
    monkeypatch.setattr(
        "devnet_tests.state_safety.subprocess.run",
        lambda *a, **k: SimpleNamespace(stdout="/opt/cronos/build/cronosd\n"),
    )

    assert _process_name(4242) == "/opt/cronos/build/cronosd"


def test_process_name_is_none_when_ps_reports_nothing(monkeypatch):
    monkeypatch.setattr(
        "devnet_tests.state_safety.Path.read_text",
        lambda self: (_ for _ in ()).throw(OSError("no /proc")),
    )
    monkeypatch.setattr(
        "devnet_tests.state_safety.subprocess.run",
        lambda *a, **k: SimpleNamespace(stdout="\n"),
    )

    assert _process_name(4242) is None
