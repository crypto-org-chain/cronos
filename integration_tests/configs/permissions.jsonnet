local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    genesis+: {
      app_state+: {
        cronos+: {
          params+: {
            cronos_admin: 'crc18z6q38mhvtsvyr5mak8fj8s8g4gw7kjjtsgrn7',  //same account as VALIDATOR2_MNEMONIC
          },
        },
      },
    },
  },
}
