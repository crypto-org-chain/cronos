std.manifestYamlDoc({
  services: {
    ['testplan-' + i]: {
      image: 'cronos-testground:d0q500phfw58nm6bygpxr2w5g67mm9fq',
      command: 'stateless-testcase run',
      container_name: 'testplan-' + i,
      volumes: [
        @'${OUTDIR:-/tmp/outputs}:/outputs',
      ],
      environment: {
        JOB_COMPLETION_INDEX: i,
      },
    }
    for i in std.range(0, 7)
  },
})
