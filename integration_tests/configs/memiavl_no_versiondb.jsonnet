// default.jsonnet with versiondb turned off on node0. Historical queries fall
// back to the versiondb multistore whenever it's enabled, which hides memiavl's
// per-query store instantiation - the path the fd-leak soak test guards.
local config = import 'default.jsonnet';
local chain = config['cronos_777-1'];

config {
  'cronos_777-1'+: {
    validators: [
      chain.validators[0] {
        'app-config'+: {
          versiondb+: {
            enable: false,
          },
        },
      },
    ] + chain.validators[1:],
  },
}
