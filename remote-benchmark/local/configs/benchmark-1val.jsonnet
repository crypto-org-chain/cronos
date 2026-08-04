// 1-validator devnet config matching the "1 Validator / Simple Transfer" setup
// described in https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
//
// Same app_patch/genesis_patch tuning as benchmark-3val.jsonnet in this
// folder, single validator, and the wiki's 1-validator mempool size (50000
// vs. 100000 for 3 validators).
{
  dotenv: '../../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace',
    // config_patch (CometBFT config.toml) from the wiki benchmark options,
    // plus testground/benchmark/benchmark/peer.py's hardcoded defaults that
    // the wiki's own "Options" JSON never lists (mempool.recheck,
    // tx_index.indexer) - those still applied for every wiki run underneath
    // whatever the Options JSON overrides.
    config: {
      db_backend: 'rocksdb',
      // not part of the wiki's benchmark options - cometbft's default
      // (~900) queues requests once a send_batch_size=8000 burst outruns it,
      // serializing at the RPC server instead of in tx processing.
      rpc: {
        'max_open_connections': 10000,
      },
      mempool: {
        size: 50000,
        recheck: false,
      },
      consensus: {
        timeout_commit: '20ms',
      },
      tx_index: {
        indexer: 'null',
      },
    },
    // app_patch (app.toml) from the wiki benchmark options
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'iavl-lazy-loading': true,
      'async-check-tx': true,
      'json-rpc': {
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
        'enable-indexer': false,
      },
      mempool: {
        'max-txs': -1,
      },
      evm: {
        'block-executor': 'block-stm',
        // 0 = auto-detect to min(GOMAXPROCS, NumCPU) (app/app.go's
        // maxParallelism()) - matches what the wiki actually ran (never
        // overridden in its app_patch), instead of a hardcoded worker count
        // that can oversubscribe smaller machines.
        'block-stm-workers': 0,
        'block-stm-pre-estimate': false,
      },
      memiavl: {
        enable: true,
        'zero-copy': true,
        'snapshot-interval': 5,
        'cache-size': 0,
        'async-commit-buffer': 16,
      },
      grpc: {
        'skip-check-header': true,
      },
    },
    validators: [
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '1000000000000000000000000000basetcro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'signer1',
        coins: '20000000000000000000000basetcro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '30000000000000000000000basetcro',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    // genesis_patch from the wiki benchmark options
    genesis: {
      consensus: {
        params: {
          block: {
            max_gas: '363000000',
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
