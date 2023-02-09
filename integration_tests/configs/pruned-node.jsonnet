local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      pruning: 'everything',
      'state-sync'+: {
        'snapshot-interval': 0,
      },
    },
    validators: [super.validators[0] {
      // don't enable versiondb, since it don't do pruning right now
      'app-config':: super['app-config'],
    }] + super.validators[1:],
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
