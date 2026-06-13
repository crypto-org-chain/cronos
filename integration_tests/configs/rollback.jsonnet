local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    // broken-cronosd test binary predates the app-side mempool ABCI; keep flood.
    config+: {
      mempool: { version: 'v1' },
    },
    validators: super.validators + [{
      name: 'rollback-test-memiavl',
      'app-config': {
        memiavl: {
          enable: true,
        },
      },
    }, {
      name: 'rollback-test-iavl',
      'app-config': {
        memiavl: {
          enable: false,
        },
      },
    }],
  },
}
