local ibc = import 'ibc.jsonnet';

ibc {
  'cronos_777-1'+: {
    genesis+: {
      app_state+: {
        cronos+: {
          params+: {
            ibc_timeout: 0,
            ibc_timeout_height: '1-1000',
          },
        },
      },
    },
  },
}
