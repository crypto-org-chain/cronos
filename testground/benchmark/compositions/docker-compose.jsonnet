std.manifestYamlDoc({
  services: {
    ['testplan-' + i]: {
      image: 'cronos-testground:latest',
      command: 'stateless-testcase run',
      container_name: 'testplan-' + i,
      volumes: [
        std.extVar('outputs') + ':/outputs',
      ],
      environment: {
        JOB_COMPLETION_INDEX: i,
      },
      ulimits: {
        nofile: {
          soft: 65536,
          hard: 65536,
        },
      },
    }
    for i in std.range(0, std.extVar('nodes') - 1)
  },
})
