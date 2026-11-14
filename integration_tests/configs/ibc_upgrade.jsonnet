local config = import 'ibc.jsonnet';

config {
  'cronos_777-1'+: {
    // Set per chain: pystarport's --cmd would replace chainmain's binary too.
    cmd: '${CRONOS_GENESIS_BINARY}',
  },
}
