{
  'cronos_777-1': {
    'start-flags': '--trace',
    cmd: 'cronosd',

    local _1mil_tcro = '1000000000000000000000000basetcro',
    local _10quintillion_stake = '10000000000000000000stake',
    local _1quintillion_stake = '1000000000000000000stake',
    local _1mil_qatest = '1000000qatest',

    validators: [
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'elbow flight coast travel move behind sister tell avocado road wait above',
        gas_prices: '100000000000000000basetcro',
        base_port: 26650,
        'app-config': {
          staking: {
            'cache-size': -1,  // disabled
          },
        },
      },
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'nasty large defy garage violin casual alarm blue marble industry infant inside',
        gas_prices: '100000000000000000basetcro',
        base_port: 26660,
        'app-config': {
          staking: {
            'cache-size': 0,  // unlimited
          },
        },
      },
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'lobster culture confirm twist oak sock lucky core kiss echo term faint robot purity fluid mix rescue music drive spot term pistol feed abuse',
        gas_prices: '100000000000000000basetcro',
        base_port: 26670,
        'app-config': {
          staking: {
            'cache-size': 1,  // size limit 1
          },
        },
      },
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'wonder grocery sing soccer two portion shift science gain tuition mean garbage feed execute brush civil buddy filter mandate aunt rocket quarter aim first',
        gas_prices: '100000000000000000basetcro',
        base_port: 26680,
        'app-config': {
          staking: {
            'cache-size': 2,  // size limit 2
          },
        },
      },
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'super develop desert oak load field ring jazz tray spray found novel',
        gas_prices: '100000000000000000basetcro',
        base_port: 26690,
        'app-config': {
          staking: {
            'cache-size': 3,  // size limit 3
          },
        },
      },
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'author satoshi neck arm afraid route carbon invite frozen drink upon point devote slow chase',
        gas_prices: '100000000000000000basetcro',
        base_port: 26700,
        'app-config': {
          staking: {
            'cache-size': 100,  // size limit 100
          },
        },
      },
      {
        coins: std.join(',', [_1mil_tcro, _10quintillion_stake]),
        staked: _1quintillion_stake,
        mnemonic: 'visual loyal reward cloud other remember sting control half flight maze unveil cherry elite carry',
        gas_prices: '100000000000000000basetcro',
        base_port: 26710,
        'app-config': {
          staking: {
            'cache-size': 1000,  // size limit 1000
          },
        },
      },
    ],
    accounts: [
      {
        name: 'rich',
        coins: std.join(',', [_1mil_tcro, _1mil_qatest, _10quintillion_stake]),
        mnemonic: 'loyal legend allow glow wheel heavy pretty example tell peasant myself garlic battle bachelor buddy stand true grit manual letter wire alone polar glove',
      },
      {
        name: 'alice',
        coins: std.join(',', [_1mil_tcro, _1mil_qatest, _10quintillion_stake]),
        mnemonic: 'style recipe economy valve curtain raw scare unable chair silly impact thrive moment copy able voyage slush diary adjust boss smile finger volume reward',
      },
      {
        name: 'bob',
        coins: std.join(',', [_1mil_tcro, _1mil_qatest, _10quintillion_stake]),
        mnemonic: 'frost worth crisp gasp this waste harbor able ethics raise december tent kid brief banner frame absent fragile police garage remind stomach side midnight',
      },
      {
        name: 'charlie',
        coins: std.join(',', [_1mil_tcro, _1mil_qatest, _10quintillion_stake]),
        mnemonic: 'worth lounge teach critic forward disease shy genuine rain gorilla end depth sort clutch museum festival stay joke custom anchor seven outside equip crawl',
      },
    ],

    config: {
      'unsafe-ignore-block-list-failure': true,
      consensus: {
        timeout_commit: '1s',
        create_empty_blocks_interval: '1s',
      },
    },

    'app-config': {
      'minimum-gas-prices': '5000000000000basetcro',
      'app-db-backend': 'goleveldb',
      pruning: 'nothing',
      rosetta: {
        'denom-to-suggest': 'basetcro',
      },
      evm: {
        'max-tx-gas-wanted': 0,
      },
      'json-rpc': {
        address: '0.0.0.0:{EVMRPC_PORT}',
        'ws-address': '0.0.0.0:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'block-range-cap': 30,
        'evm-timeout': '10s',
      },
      'blocked-addresses': [],
      mempool: {
        'max-txs': 0,
      },
    },
    genesis: {
      consensus: {
        params: {
          block: {
            max_bytes: '1048576',
            max_gas: '81500000',
          },
          evidence: {
            max_age_num_blocks: '403200',
            max_age_duration: '2419200000000000',
            max_bytes: '150000',
          },
        },
      },
      app_state: {
        bank: {
          send_enabled: [
            {
              denom: 'stake',
              enabled: true,
            },
            {
              denom: 'basetcro',
              enabled: false,
            },
          ],
        },
        cronos: {
          params: {
            cronos_admin: 'crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp',
            ibc_cro_denom: 'ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865',
          },
        },
        distribution: {
          params: {
            community_tax: '0',
            base_proposer_reward: '0',
            bonus_proposer_reward: '0',
          },
        },
        evm: {
          params: {
            evm_denom: 'basetcro',
          },
        },
        gov: {
          params: {
            min_deposit: [
              {
                denom: 'basetcro',
                amount: '5',
              },
            ],
            max_deposit_period: '30s',
            voting_period: '30s',
            expedited_voting_period: '15s',
            expedited_min_deposit: [
              {
                denom: 'basetcro',
                amount: '25',
              },
            ],
          },
        },
        ibc: {
          client_genesis: {
            params: {
              allowed_clients: [
                '06-solomachine',
                '07-tendermint',
                '09-localhost',
              ],
            },
          },
        },
        mint: {
          minter: {
            inflation: '0.000000000000000000',
            annual_provisions: '0.000000000000000000',
          },
          params: {
            inflation_rate_change: '0',
            inflation_max: '0',
            inflation_min: '0',
            goal_bonded: '1',
          },
        },
        slashing: {
          params: {
            downtime_jail_duration: '60s',
            min_signed_per_window: '0.5',
            signed_blocks_window: '10',
            slash_fraction_double_sign: '0',
            slash_fraction_downtime: '0',
          },
        },
        staking: {
          params: {
            unbonding_time: '60s',
            max_validators: '50',
          },
        },
        feemarket: {
          // from https://rest-t3.cronos.org/ethermint/feemarket/v1/params
          params: {
            no_base_fee: false,
            base_fee_change_denominator: 100,
            elasticity_multiplier: 4,
            // enabled at genesis, different from testnet
            enable_height: '0',
            // initial base fee at genesis, testnet shows the current base fee, hence different
            base_fee: '1000000000',
            min_gas_price: '1000000000',
            min_gas_multiplier: '0.500000000000000000',
          },
        },
      },
    },
  },
}

