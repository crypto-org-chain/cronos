local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      preconfer: {
        // Enable priority transaction support
        enable: true,
        // Optional: Set validator address for signing preconfirmations
        // validator_address: 'cronosvaloper1...',
        // Optional: Configure preconfirmation timeout (default: "30s")
        preconfirm_timeout: '45s',
        // Optional: Whitelist addresses allowed to use priority boosting
        // Empty list = all addresses allowed (default)
        // whitelist: [
        //   '0x1234567890123456789012345678901234567890',
        //   '0xABCDEF1234567890ABCDEF1234567890ABCDEF12',
        // ],
      },
    },
    validators: [
      super.validators[0] {
        'app-config'+: {
          preconfer: {
            enable: true,
            preconfirm_timeout: '30s',
          },
        },
      },
    ] + super.validators[1:],
  },
}

