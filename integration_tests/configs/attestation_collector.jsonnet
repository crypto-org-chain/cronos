local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'account-prefix': 'crc',
    'coin-type': 60,
    key_name: 'signer1',
    'app-config'+: {
      'minimum-gas-prices': '0basecro',
      'index-events': super['index-events'] + ['message.action'],
    },
    // Override accounts to use basecro instead of basetcro
    accounts: [
      {
        name: 'community',
        coins: '10000000000000000000000basecro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'signer1',
        coins: '20000000000000000000000basecro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '30000000000000000000000basecro,100000000000ibcfee',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    genesis+: {
      app_state+: {
        evm+: {
          params+: {
            evm_denom: 'basecro',
          },
        },
        cronos+: {
          params+: {
            max_callback_gas: 50000,
          },
        },
        feemarket+: {
          params+: {
            no_base_fee: true,
            base_fee: '0',
          },
        },
        // Attestation module configuration
        attestation: {
          params: {
            attestation_enabled: true,
            attestation_interval: 10,  // Send attestation every 10 blocks
            packet_timeout_timestamp: 600000000000,  // 10 minutes
          },
        },
      },
    },
  },
}

