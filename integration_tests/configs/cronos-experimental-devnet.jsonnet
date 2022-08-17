local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'cmd-flags': '--unsafe-experimental',
    'start-flags': '--trace',
    config:: super['config'],
    'app-config'+: {
      'app-db-backend':: super['app-db-backend'],
      'minimum-gas-prices': '100000000000basetcro',
      'json-rpc': {
        address: '0.0.0.0:{EVMRPC_PORT}',
        'ws-address': '0.0.0.0:{EVMRPC_PORT_WS}',
      },
    },
    genesis+: {
      app_state+: {
        cronos: {
          params: {
            cronos_admin: 'crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp',
            enable_auto_deployment: true,
            ibc_cro_denom: 'ibc/6411AE2ADA1E73DB59DB151A8988F9B7D5E7E233D8414DB6817F8F1A01611F86',
          },
        },
      },
      consensus_params+: {
        block+: {
          time_iota_ms: '2000',
        },
      },
    },
  },
}
