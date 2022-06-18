local config = import 'default.jsonnet';
local Utils = import 'utils.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'minimum-gas-prices': '5000000000000basetcro',
      'json-rpc'+: {
        api:: super['api'],
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
      },
    },
    genesis+: {
      app_state+: {
        evm+: {
          params+: {
            chain_config: {
              london_block: null,
            },
          },
        },
        feemarket: {
          params: {
            no_base_fee: true,
          },
        },
      },
    },
  },
}
