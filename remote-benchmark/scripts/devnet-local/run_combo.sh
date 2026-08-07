#!/usr/bin/env bash
# Driver: run 3 rounds of one (binary, validators, testcase) combo via
# run-benchmark.sh, append each round's overall_tps (or NA on failure) to
# report/matrix-results.csv. Scratch helper for the wiki-comparison matrix,
# not part of the shipped benchmark suite.
set -uo pipefail

BIN_PATH="$1"
BIN_LABEL="$2"
VALIDATORS="$3"
TESTCASE="$4"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# report/ is generated output, kept alongside the devnet binaries under
# remote-benchmark/local/ rather than moving with this script.
LOCAL_ARTIFACTS_DIR="$(cd "${SCRIPT_DIR}/../../local" && pwd)"
CSV="${LOCAL_ARTIFACTS_DIR}/report/matrix-results.csv"
LOGDIR="${LOCAL_ARTIFACTS_DIR}/report/matrix-logs"
mkdir -p "${LOGDIR}"

for round in 1 2 3; do
  LOG="${LOGDIR}/${BIN_LABEL}-${VALIDATORS}val-${TESTCASE}-r${round}.log"
  echo "=== ${BIN_LABEL} ${VALIDATORS}val ${TESTCASE} round ${round}/3 ===" >&2
  if CRONOS_BIN="${BIN_PATH}" "${SCRIPT_DIR}/run-benchmark.sh" "${VALIDATORS}" "${TESTCASE}" >"${LOG}" 2>&1; then
    TPS="$(grep -m1 '^overall_tps ' "${LOG}" | awk '{print $2}')"
    if [[ -n "${TPS}" ]]; then
      echo "${BIN_LABEL},${VALIDATORS},${TESTCASE},${round},${TPS},ok,${LOG}" >> "${CSV}"
      echo "  -> overall_tps=${TPS}" >&2
    else
      echo "${BIN_LABEL},${VALIDATORS},${TESTCASE},${round},NA,ok-no-tps,${LOG}" >> "${CSV}"
      echo "  -> succeeded but no overall_tps line found" >&2
    fi
  else
    echo "${BIN_LABEL},${VALIDATORS},${TESTCASE},${round},NA,failed,${LOG}" >> "${CSV}"
    echo "  -> FAILED (see ${LOG})" >&2
  fi
done
