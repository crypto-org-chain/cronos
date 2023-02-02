local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'iavl-disable-fastnode': true,
    },
    validators: super.validators + [{
      name: 'fullnode',
    }],
  },
}
