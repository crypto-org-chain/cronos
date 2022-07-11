local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'start-flags': '--trace --unsafe-experimental',
    'app-config'+: {
      'minimum-gas-prices':: super['minimum-gas-prices'],
      'json-rpc'+: {
        api:: super['api'],
      },
    },
    accounts: [{
      name: 'community',
      coins: '10000000000000000000000basetcro',
      mnemonic: '${COMMUNITY_MNEMONIC}',
    }],
    genesis+: {
      consensus_params:: super['consensus_params'],
      app_state+: {
        cronos: {
          params: {
            cronos_admin: 'crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp',
            enable_auto_deployment: false,
          },
        },
        transfer:: super['transfer'],
      },
    },
  },
}
