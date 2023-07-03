local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    validators: super.validators + [{
      name: 'fullnode',
      'app-config': {
        memiavl: {
          enable: true,
        },
      },
    }],
  },
}
