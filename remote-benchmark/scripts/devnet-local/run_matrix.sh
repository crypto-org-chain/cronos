#!/usr/bin/env bash
# Master driver for the full wiki-comparison matrix: for each binary x
# validators x testcase, run 3 rounds via run_combo.sh. Scratch helper, not
# part of the shipped benchmark suite.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HEAD_BIN="/Users/jaytseng/workspace/cronos/cronosd"
V178_BIN="/Users/jaytseng/workspace/cronos/remote-benchmark/local/cronos_1.7.8_Darwin_arm64/bin/cronosd"

TESTCASES=(simple-transfer erc20-transfer batch-simple-transfer batch-erc20-transfer)
VALIDATOR_COUNTS=(1 3 5)

for BIN_LABEL_PATH in "head:${HEAD_BIN}" "v1.7.8:${V178_BIN}"; do
  BIN_LABEL="${BIN_LABEL_PATH%%:*}"
  BIN_PATH="${BIN_LABEL_PATH#*:}"
  for VALIDATORS in "${VALIDATOR_COUNTS[@]}"; do
    for TESTCASE in "${TESTCASES[@]}"; do
      echo "##### combo: ${BIN_LABEL} ${VALIDATORS}val ${TESTCASE} #####"
      "${SCRIPT_DIR}/run_combo.sh" "${BIN_PATH}" "${BIN_LABEL}" "${VALIDATORS}" "${TESTCASE}"
    done
  done
done

echo "##### matrix complete #####"
