#!/usr/bin/env bash
# Raise UDP buffer ceilings on the Docker host kernel so libp2p/QUIC stops
# logging "failed to sufficiently increase receive buffer size" warnings.
#
# net.core.{r,w}mem_max are non-namespaced — they cannot be set inside a
# container, even with --privileged. They must be set on the host kernel.
# On macOS Docker Desktop the "host" is the LinuxKit VM; this script reaches
# into that VM via a privileged --pid=host helper.
#
# Run once per Docker Desktop start (settings do not persist across VM restart).
set -euo pipefail

RMEM=${RMEM:-7500000}
WMEM=${WMEM:-7500000}

case "$(uname -s)" in
  Linux)
    if [[ $EUID -ne 0 ]]; then
      exec sudo -E "$0" "$@"
    fi
    sysctl -w net.core.rmem_max="$RMEM"
    sysctl -w net.core.wmem_max="$WMEM"
    ;;
  Darwin)
    docker run --rm --privileged --pid=host alpine \
      nsenter -t 1 -m -u -i -n sysctl -w \
        "net.core.rmem_max=$RMEM" "net.core.wmem_max=$WMEM"
    ;;
  *)
    echo "unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

echo "applied: net.core.rmem_max=$RMEM net.core.wmem_max=$WMEM"
