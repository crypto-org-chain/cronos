std.manifestYamlDoc({
  services: {
    ['testplan-' + i]: {
      image: 'ghcr.io/crypto-org-chain/cronos-testground:latest',
      command: 'stateless-testcase run /data 3 --num_accounts=10 --num_txs=1000',
      container_name: 'testplan-' + i,
      volumes: [
        @'${DATADIR:-/tmp/data/out}:/data',
      ],
      environment: {
        JOB_COMPLETION_INDEX: i,
      },
    }
    for i in std.range(0, 9)
  },
})
