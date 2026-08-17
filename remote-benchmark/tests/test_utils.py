from remote_benchmark import utils as utils_module
from remote_benchmark.utils import (
    blockchain_range,
    eth_to_bech32,
    request_json,
    split,
    split_batch,
)


def test_eth_to_bech32_encodes_with_the_cronos_prefix():
    assert (
        eth_to_bech32("0x1234567890123456789012345678901234567890")
        == "crc1zg69v7yszg69v7yszg69v7yszg69v7ysz9muj2"
    )


def test_split():
    assert split(10, 3) == [(0, 4), (4, 7), (7, 10)]


def test_split_batch():
    assert split_batch(10, 3) == [(0, 3), (3, 6), (6, 9), (9, 10)]


def _stub_session(monkeypatch, get=None, post=None):
    """Replace _get_session with a fake exposing the given get/post, so
    request_json's session-based dispatch can be tested without hitting a
    real socket - a plain requests.get/post monkeypatch is no longer enough
    since request_json now calls through a persistent Session for keep-alive."""

    class FakeSession:
        pass

    fake = FakeSession()
    if get is not None:
        fake.get = get
    if post is not None:
        fake.post = post
    monkeypatch.setattr(utils_module, "_get_session", lambda base: fake)
    return fake


def test_blockchain_range_chunks_by_page_size(monkeypatch):
    # A 45-height span should page in 20/20/5, not one call per height.
    requested = []

    class FakeResponse:
        def __init__(self, lo, hi):
            self._lo, self._hi = lo, hi

        def json(self):
            return {
                "result": {
                    "block_metas": [
                        {"header": {"height": str(h), "time": f"t{h}"}, "num_txs": str(h % 3)}
                        for h in range(self._lo, self._hi + 1)
                    ]
                }
            }

    def fake_get(url, timeout):
        # url looks like "{rpc}/blockchain?minHeight={lo}&maxHeight={hi}"
        query = url.split("?", 1)[1]
        params = dict(pair.split("=") for pair in query.split("&"))
        lo, hi = int(params["minHeight"]), int(params["maxHeight"])
        requested.append((lo, hi))
        return FakeResponse(lo, hi)

    _stub_session(monkeypatch, get=fake_get)

    metas = blockchain_range(1, 45, "http://rpc")

    assert sorted(requested) == [(1, 20), (21, 40), (41, 45)]
    assert len(metas) == 45
    assert metas[41] == (41 % 3, "t41")


def test_request_json_single_rpc_still_works(monkeypatch):
    class FakeResponse:
        def json(self):
            return {"ok": True}

    requested = []

    def fake_get(url, timeout):
        requested.append(url)
        return FakeResponse()

    _stub_session(monkeypatch, get=fake_get)

    assert request_json(utils_module.requests.get, "http://rpc", "/status") == {"ok": True}
    assert requested == ["http://rpc/status"]


def test_request_json_round_robins_across_pool(monkeypatch):
    class FakeResponse:
        def json(self):
            return {"ok": True}

    requested = []

    def fake_get(url, timeout):
        requested.append(url)
        return FakeResponse()

    _stub_session(monkeypatch, get=fake_get)

    candidates = ["http://a", "http://b", "http://c"]
    for _ in range(6):
        request_json(utils_module.requests.get, candidates, "/status")

    bases = [url.rsplit("/status", 1)[0] for url in requested]
    assert bases == candidates * 2
