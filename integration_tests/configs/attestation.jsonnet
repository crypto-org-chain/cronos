// Configuration for attestation integration test
// Sets up Cronos chain and Attestation Layer chain with Hermes relayer

local config = import 'default.jsonnet';

config {
  // Cronos chain configuration
  'cronos_777-1'+: {
    'account-prefix': 'crc',
    'coin-type': 60,
    key_name: 'signer1',
    config+: {

    },
    'app-config'+: {
      'index-events': super['index-events'] + ['message.action'],
    },
    genesis+: {
      app_state+: {
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
  // Attestation Layer chain configuration
  'attestation-1': {
    cmd: 'cronos-attestad',
    'start-flags': '--trace',
    'key-name': 'validator',
    'keyring-backend': 'test',
    'account-prefix': 'cro',
    'coin-type': 394,
    config: {
      consensus: {
        'timeout-commit': '1s',
      },
    },
    'app-config': {
      'chain_id': 'attestation-1',
      'api': {
        enable: true,
      },
      'minimum-gas-prices': '0stake',
      'iavl-lazy-loading': true,
    },
    validators: [
      {
        coins: '2234240000000000000cro',
        staked: '10000000000000cro',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        base_port: 26800,
      },
      {
        coins: '987870000000000000cro',
        staked: '20000000000000cro',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        base_port: 26810,
      },
    ],
    accounts: [
      {
        name: 'relayer',
        coins: '10000000000000cro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '10000000000000cro,100000000000ibcfee',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    genesis: {
      app_state: {
        transfer: {
          params: {
            receive_enabled: true,
            send_enabled: true,
          },
        },
        cronosattesta: {
          params: {
            // Attestation layer specific params
          },
        },
      },
    },
  },
  
  relayer: {
    mode: {
      clients: {
        enabled: true,
        refresh: true,
        misbehaviour: false,
      },
      connections: {
        enabled: false,  // V2 doesn't use connections
      },
      channels: {
        enabled: false,  // V2 doesn't use channels
      },
      packets: {
        enabled: true,
        tx_confirmation: true,
      },
    },
    rest: {
      enabled: true,
      host: '127.0.0.1',
      port: 3000,
    },
    chains: [
      {
        id: 'cronos_777-1',
        max_gas: 2500000,
        gas_multiplier: 1.1,
        address_type: {
          derivation: 'ethermint',
          proto_type: {
            pk_type: '/ethermint.crypto.v1.ethsecp256k1.PubKey',
          },
        },
        gas_price: {
          price: 10000000,
          denom: 'basecro',
        },
        event_source: {
          batch_delay: '5000ms',
        },
        extension_options: [{
          type: 'ethermint_dynamic_fee',
          value: '1000000',
        }],
      },
      {
        id: 'attestation-1',
        max_gas: 500000,
        gas_price: {
          price: 1000000,
          denom: 'basecro',
        },
        event_source: {
          batch_delay: '5000ms',
        },
      },
    ],
  },
}

