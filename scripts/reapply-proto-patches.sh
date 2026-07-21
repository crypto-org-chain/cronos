#!/usr/bin/env bash
#
# Re-applies hand-maintained security patches to generated protobuf code that
# `make proto-gen` would otherwise silently wipe.

set -eo pipefail

QUERY_PB_GO="x/cronos/types/query.pb.go"
MARKER="PROTOCGEN-PATCH:replay-block-msgs-cap"

if grep -q "$MARKER" "$QUERY_PB_GO"; then
  echo "reapply-proto-patches: $MARKER already present, skipping"
  exit 0
fi

python3 - "$QUERY_PB_GO" "$MARKER" <<'EOF'
import sys

path, marker = sys.argv[1], sys.argv[2]
target = "\t\t\tm.Msgs = append(m.Msgs, &types.MsgEthereumTx{})\n"
patch = (
    "\t\t\t// Hand-maintained cap (see query.go): reject before appending, so a\n"
    "\t\t\t// huge attacker batch can't OOM us before the keeper ever runs.\n"
    f"\t\t\t// {marker}\n"
    "\t\t\tif len(m.Msgs) >= MaxReplayBlockMsgs {\n"
    "\t\t\t\treturn fmt.Errorf(\"proto: ReplayBlockRequest.Msgs exceeds max allowed count %d\", MaxReplayBlockMsgs)\n"
    "\t\t\t}\n"
    + target
)

with open(path) as f:
    content = f.read()

count = content.count(target)
if count != 1:
    sys.exit(f"reapply-proto-patches: expected exactly 1 occurrence of append target in {path}, found {count}")

content = content.replace(target, patch, 1)

with open(path, "w") as f:
    f.write(content)

print(f"reapply-proto-patches: applied {marker} to {path}")
EOF
