local config = import 'min_gas_price_lte.jsonnet';

config {
  'cronos_777-1'+: {
    genesis+: {
      consensus_params+: {
        block+: {
          max_gas: '84000000',
        },
      },
    },
  },
}
