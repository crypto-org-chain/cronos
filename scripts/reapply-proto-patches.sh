#!/bin/sh
#
# Re-applies hand-maintained security patches to generated protobuf code that
# `make proto-gen` would otherwise silently wipe.

set -e

QUERY_PB_GO="x/cronos/types/query.pb.go"
MARKER="PROTOCGEN-PATCH:replay-block-msgs-cap"
TARGET='			m.Msgs = append(m.Msgs, &types.MsgEthereumTx{})'

if grep -q "$MARKER" "$QUERY_PB_GO"; then
  echo "reapply-proto-patches: $MARKER already present, skipping"
  exit 0
fi

# Single pass: the same $0 == target check both counts and drives the
# substitution, so the count check can never disagree with what actually
# gets patched.
awk -v marker="$MARKER" -v target="$TARGET" '
  $0 == target {
    matched++
    print "\t\t\t// Hand-maintained cap (see query.go): reject before appending, so a"
    print "\t\t\t// huge attacker batch can'"'"'t OOM us before the keeper ever runs."
    print "\t\t\t// " marker
    print "\t\t\tif len(m.Msgs) >= MaxReplayBlockMsgs {"
    print "\t\t\t\treturn fmt.Errorf(\"proto: ReplayBlockRequest.Msgs exceeds max allowed count %d\", MaxReplayBlockMsgs)"
    print "\t\t\t}"
  }
  { print }
  END {
    if (matched != 1) {
      print "reapply-proto-patches: expected exactly 1 occurrence of append target, found " matched + 0 > "/dev/stderr"
      exit 1
    }
  }
' "$QUERY_PB_GO" > "${QUERY_PB_GO}.tmp" && mv "${QUERY_PB_GO}.tmp" "$QUERY_PB_GO" || { rm -f "${QUERY_PB_GO}.tmp"; exit 1; }

echo "reapply-proto-patches: applied $MARKER to $QUERY_PB_GO"
