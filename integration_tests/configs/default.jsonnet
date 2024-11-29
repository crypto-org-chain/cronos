{
  dotenv: '../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace',
    config: {
      db_backend: 'rocksdb',
      mempool: {
        version: 'v1',
      },
    },
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'iavl-lazy-loading': true,
      'json-rpc': {
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
      },
      evm: {
        'block-executor': 'sequential',
      },
      mempool: {
        'max-txs': 1000,
      },
    },
    validators: [{
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${VALIDATOR1_MNEMONIC}',
      client_config: {
        'broadcast-mode': 'sync',
      },
      'app-config': {
        memiavl: {
          enable: true,
          'zero-copy': true,
          'snapshot-interval': 5,
          'cache-size': 0,
        },
        versiondb: {
          enable: true,
        },
        evm: {
          'block-executor': 'block-stm',
          'block-stm-workers': 32,
        },
      },
    }, {
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${VALIDATOR2_MNEMONIC}',
      client_config: {
        'broadcast-mode': 'sync',
      },
      config: {
        db_backend: 'pebbledb',
      },
      'app-config': {
        'app-db-backend': 'pebbledb',
      },
    }, {
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${VALIDATOR3_MNEMONIC}',
      client_config: {
        'broadcast-mode': 'sync',
      },
      config: {
        db_backend: 'goleveldb',
      },
      'app-config': {
        'app-db-backend': 'goleveldb',
      },
    }],
    accounts: [{
      name: 'community',
      coins: '10000000000000000000000basetcro',
      mnemonic: '${COMMUNITY_MNEMONIC}',
    }, {
      name: 'signer1',
      coins: '20000000000000000000000basetcro',
      mnemonic: '${SIGNER1_MNEMONIC}',
    }, {
      name: 'signer2',
      coins: '30000000000000000000000basetcro',
      mnemonic: '${SIGNER2_MNEMONIC}',
    }],
    genesis: {
      consensus: {
        params: {
          block: {
            max_bytes: '1048576',
            max_gas: '81500000',
          },
        },
      },
      app_state: {
        evm: {
          params: {
            evm_denom: 'basetcro',
          },
        },
        cronos: {
          params: {
            cronos_admin: '${CRONOS_ADMIN}',
            enable_auto_deployment: true,
            ibc_cro_denom: '${IBC_CRO_DENOM}',
          },
        },
        e2ee: {
          keys: [{
            address: 'crc16z0herz998946wr659lr84c8c556da55dc34hh',
            key: 'age1k3mpspxytgvx6e0jja0xgrtzz7vw2p00c2a3xmq5ygfzhwh4wg0s35z4c8',
          }],
        },
        gov: {
          params: {
            expedited_voting_period: '1s',
            voting_period: '10s',
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basetcro',
                amount: '1',
              },
            ],
          },
        },
        transfer: {
          params: {
            receive_enabled: true,
            send_enabled: true,
          },
        },
        feemarket: {
          params: {
            no_base_fee: false,
            base_fee: '100000000000',
          },
        },
      },
    },
  },
}
