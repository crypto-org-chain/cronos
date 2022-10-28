local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'cmd-flags': '--unsafe-experimental',
    'app-config'+: {
      'minimum-gas-prices': '100000000000basetcro',
    },
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            no_base_fee: true,
          },
        },
      },
    },
  },
}
