local config = import 'default.jsonnet';
local Utils = import 'utils.jsonnet';

config {
  'cronos_777-1'+: {
    validators: Utils.validators_with_timeout([
      '${VALIDATOR1_MNEMONIC}',
      '${VALIDATOR2_MNEMONIC}',
    ]),
  },
}
