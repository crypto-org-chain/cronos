local config = import 'default.jsonnet';
local Utils = import 'utils.jsonnet';

config {
  'cronos_777-1'+: {
    config: {
      consensus: {
        'timeout_commit': '15s',
      },
    },
  },
}
