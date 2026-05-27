#!/usr/bin/env bash

set -euo pipefail

# Bump the ethermint replace directive to the latest commit on develop.
# Dependabot cannot track pseudo-version dependencies on branches, so this
# script is used from CI instead.

ETHERMINT_REPO=github.com/crypto-org-chain/ethermint
ETHERMINT_REPLACE_LEFT=github.com/evmos/ethermint
BRANCH=develop

COMMIT=$(
  git ls-remote "https://${ETHERMINT_REPO}.git" "refs/heads/${BRANCH}" \
    | awk '{print $1}'
)
if [[ -z "$COMMIT" ]]; then
  echo "failed to resolve ${BRANCH} ref for ${ETHERMINT_REPO}" >&2
  exit 1
fi

VERSION=$(
  curl -fsSL "https://proxy.golang.org/${ETHERMINT_REPO}/@v/${COMMIT}.info" \
    | jq -r .Version
)
if [[ -z "$VERSION" || "$VERSION" == "null" ]]; then
  echo "failed to resolve module version for ${COMMIT}" >&2
  exit 1
fi

CURRENT=$(
  grep -E "${ETHERMINT_REPLACE_LEFT} =>" go.mod \
    | grep -oE 'v[0-9].+' \
    | head -1 \
    || true
)
if [[ "$CURRENT" == "$VERSION" ]]; then
  echo "ethermint already at ${VERSION} (${COMMIT})"
  exit 0
fi

echo "updating ethermint replace from ${CURRENT:-unknown} to ${VERSION} (${COMMIT})"
go mod edit \
  -replace="${ETHERMINT_REPLACE_LEFT}=${ETHERMINT_REPO}@${VERSION}"
