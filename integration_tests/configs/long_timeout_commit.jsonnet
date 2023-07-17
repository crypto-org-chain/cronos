local default = import 'default.jsonnet';

default {
  'cronos_777-1'+: {
    config+: {
      consensus+: {
        timeout_commit: '15s',
      },
    },
    'app-config'+: {
      'blocked-addresses': ['crc16z0herz998946wr659lr84c8c556da55dc34hh'],
    },
  },
}
