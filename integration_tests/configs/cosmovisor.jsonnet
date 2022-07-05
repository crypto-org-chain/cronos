local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'minimum-gas-prices': '5000000000000basetcro',
    },
    genesis+: {
      app_state+: {
        feemarket: {
          params: {
            no_base_fee: true,
          },
        },
      },
    },
  },
}
