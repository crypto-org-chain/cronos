# Remote 5-validator devnet benchmark plan (v1.7.8 vs v1.8.0-alpha)

Intra-VPC TCP+UDP 26650-26699 opened between the 5 hosts (per `hosts.env`).
This is the runbook to go from that to a comparable before/after benchmark.

## 1. Verify connectivity (read-only)

- SSH reachable on all 5 public IPs.
- Port range 26650-26699 actually open node-to-node (TCP p2p/rpc/json-rpc/ws;
  UDP for libp2p if a QUIC/UDP transport is enabled).
- Confirm node0's existing binaries (`NODE1_V178_BIN`, `NODE1_V180A_BIN`) and
  `librocksdb`/`libsnappy` libs are still present.

## 2. Per-tag (v178, v180a) genesis+config prep, done locally on this Mac

- `pystarport init` locally with the 5-val jsonnet config (Darwin arm64
  binaries already local) -> produces a chain-dir with genesis + per-node
  config/app.toml.
- Run `patch-remote-config.py <chain-dir>` -- rewrites loopback binds to
  `0.0.0.0`, sets `external_address` to each node's private IP, rewrites
  `persistent_peers`, patches libp2p bootstrap_peers if enabled.

## 3. Stage the Linux binary

- scp the linux/aarch64 binary down from node0 (owns both builds per
  `hosts.env`) to a local tmp path.

## 4. Deploy

- `deploy.sh <tag> libs` -- push rocksdb/snappy libs to nodes 1-4 (skip for
  v178, static binary).
- `deploy.sh <tag> push <chain-dir> <local-linux-binary>` -- ships binary +
  per-node home to all 5, verifies genesis sha256 matches everywhere.
- `deploy.sh <tag> start "<flags>"` -- flags come from the locally generated
  tasks.ini (honors v1.8's `--async-check-tx` vs v1.7.8's absence
  automatically).
- `deploy.sh <tag> health` -- checks `/status` not catching up,
  `n_peers==4`, `eth_blockNumber` answers on all 5.

## 5. Run the load

- `remote-benchmark preflight` first (mempool.type + peer matrix across all
  5 RPCs).
- `fund` -> `check` -> `bench`/`soak` against a config pointing at the 5
  public IPs' rpc/json-rpc ports (26657/26667/.../26697,
  26651/26661/.../26691).
- Repeat for both tags, same account funding, same load shape, for a clean
  before/after.

## 6. Teardown

- `deploy.sh <tag> stop` after each tag's run, `wipe` when fully done.
