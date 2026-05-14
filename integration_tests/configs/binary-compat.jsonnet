// Configuration for binary compatibility testing
// This is a simplified config with 3 validators for testing mixed binary versions
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
    },
    // 3 validators for binary compatibility testing
    validators: [{
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${VALIDATOR1_MNEMONIC}',
    }, {
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${VALIDATOR2_MNEMONIC}',
    }, {
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${VALIDATOR3_MNEMONIC}',
    }],
    accounts: [{
      name: 'community',
      coins: '10000000000000000000000basetcro',
      mnemonic: '${COMMUNITY_MNEMONIC}',
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
        gov: {
          params: {
            expedited_voting_period: '10s',
            voting_period: '30s',
            max_deposit_period: '30s',
            min_deposit: [
              {
                denom: 'basetcro',
                amount: '1',
              },
            ],
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

