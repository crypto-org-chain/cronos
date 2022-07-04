local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      evm+: {
        'max-tx-gas-wanted': 0,
      },
    },
    genesis+: {
      consensus_params+: {
        block+: {
          max_gas: '815000',
        },
      },
    },
  },
}
