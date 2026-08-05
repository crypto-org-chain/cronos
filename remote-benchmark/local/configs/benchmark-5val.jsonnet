// 5-validator devnet config matching the "5 Validators" setup described in
// https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
//
// Extends benchmark-3val.jsonnet with 2 more validators; the wiki does not
// list separate mempool/gas options for 5 validators, so all non-validator
// fields are kept identical to the 3-validator config.
{
  dotenv: '../../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace --async-check-tx',
    config: {
      db_backend: 'rocksdb',
      event_bus_buffer_capacity: 128,
      rpc: {
        'max_open_connections': 10000,
      },
      mempool: {
        size: 100000,
        recheck: false,
        type: 'app',
        broadcast: true,
        reap_max_gas: 363000000,
        reap_interval: '500ms',
      },
      consensus: {
        timeout_commit: '20ms',
      },
      tx_index: {
        indexer: 'null',
      },
      p2p: {
        libp2p: {
          enabled: true,
          // lp2p.Peer.Send opens a new libp2p stream per envelope (no reuse), so
          // sustained mempool gossip + consensus messaging across 5 peers quickly
          // exceeds go-libp2p's default per-peer stream cap; the receiver then
          // resets streams mid-write, dropping the message and stalling consensus.
          // Raise the cap well above what this benchmark's fan-out can produce.
          limits: {
            mode: 'custom',
            max_peers: 16,
            max_peer_streams: 4096,
          },
        },
      },
    },
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
        'max-txs': 100000,
        recheck: false,
        type: 'app',
        broadcast: true,
        reap_max_gas: 363000000,
        reap_interval: '500ms',
      },
      evm: {
        'block-executor': 'block-stm',
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
      cronos: {
        'mempool-txs-per-block': 17285,
        'tx-cache-size': 100000,
      },
    },
    // 5 uniform validators, all rocksdb-backed
    validators: [
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR3_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR4_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR5_MNEMONIC}',
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
