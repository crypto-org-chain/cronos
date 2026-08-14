"""Force allow_duplicate_ip=true onto [p2p] in every node's config.toml.

pystarport (pinned to tomlkit==0.7.2) mis-scopes trailing [p2p] keys when a
jsonnet config's p2p.libp2p block gets merged in: tomlkit inserts the new
[p2p.libp2p] table header in the middle of [p2p]'s existing keys instead of
at the end, silently reparenting everything after it (including
allow_duplicate_ip) under [p2p.libp2p]. cometbft then never sees
p2p.allow_duplicate_ip, falls back to its compiled-in default of false, and
same-host multi-validator devnets get capped at ~1 peer connection each.

Inserting a correctly-scoped allow_duplicate_ip = true right after the
[p2p] header sidesteps the bug regardless of where tomlkit put the original
key - TOML permits the same key name in different tables.

Some configs aren't affected by the bug at all (tomlkit scoped the key
correctly). Inserting unconditionally there would create a duplicate
allow_duplicate_ip key within the same [p2p] table, which go-toml rejects
as a parse error - so this only inserts when the key is missing from the
[p2p] block.
"""

import re
import sys
from pathlib import Path

P2P_HEADER_RE = re.compile(r"(?m)^\[p2p\]\s*\n")
NEXT_TABLE_RE = re.compile(r"(?m)^\[")
ALLOW_DUPLICATE_IP_RE = re.compile(r"(?m)^allow_duplicate_ip\s*=")


def fix_config(config_toml: Path) -> None:
    text = config_toml.read_text()
    header_match = P2P_HEADER_RE.search(text)
    if not header_match:
        return
    block_end_match = NEXT_TABLE_RE.search(text, header_match.end())
    block_end = block_end_match.start() if block_end_match else len(text)
    if ALLOW_DUPLICATE_IP_RE.search(text, header_match.end(), block_end):
        return
    text = text[: header_match.end()] + "allow_duplicate_ip = true\n" + text[header_match.end() :]
    config_toml.write_text(text)


def main() -> None:
    chain_dir = Path(sys.argv[1])
    for config_toml in sorted(chain_dir.glob("node*/config/config.toml")):
        fix_config(config_toml)


if __name__ == "__main__":
    main()
