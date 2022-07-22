local default = import 'default.jsonnet';

default {
  'cronos_777-1'+: {
    config: {
      consensus: {
        timeout_commit: '15s',
      },
    },
  },
}
