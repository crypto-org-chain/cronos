local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    // don't enable versiondb, since it don't do pruning right now
    'start-flags': '--trace --streamers file',
    'app-config'+: {
      pruning: 'everything',
      'state-sync'+: {
        'snapshot-interval': 0,
      },
    },
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            min_gas_multiplier: '0',
          },
        },
      },
    },
  },
}
