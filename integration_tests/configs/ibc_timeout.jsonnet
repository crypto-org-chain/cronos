local ibc = import 'ibc.jsonnet';

ibc {
  'cronos_777-1'+: {
    genesis+: {
      app_state+: {
        cronos+: {
          params+: {
            ibc_timeout: 1,
          },
        },
      },
    },
  },
}
