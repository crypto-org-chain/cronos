rm -rf /tmp/cronos-benchmark-3val/*


pystarport init \
    --config integration_tests/configs/benchmark-3val.jsonnet \
    --data /tmp/cronos-benchmark-3val \
    --base_port 26650 \
    --no_remove



pystarport start --data /tmp/cronos-benchmark-3val