import binascii
import enum
import hashlib
import json
import os
import subprocess
import tempfile

import bech32
from dateutil.parser import isoparse
from pystarport.utils import build_cli_args_safe, format_doc_string, interact

# the default initial base fee used by integration tests
DEFAULT_GAS_PRICE = "100000000000basetcro"


class ModuleAccount(enum.Enum):
    FeeCollector = "fee_collector"
    Mint = "mint"
    Gov = "gov"
    Distribution = "distribution"
    BondedPool = "bonded_tokens_pool"
    NotBondedPool = "not_bonded_tokens_pool"
    IBCTransfer = "transfer"


@format_doc_string(
    options=",".join(v.value for v in ModuleAccount.__members__.values())
)
def module_address(name):
    """
    get address of module accounts

    :param name: name of module account, values: {options}
    """
    data = hashlib.sha256(ModuleAccount(name).value.encode()).digest()[:20]
    return bech32.bech32_encode("cro", bech32.convertbits(data, 8, 5))


class ChainCommand:
    def __init__(self, cmd):
        self.cmd = cmd

    def __call__(self, cmd, *args, stdin=None, stderr=subprocess.STDOUT, **kwargs):
        "execute chain-maind"
        args = " ".join(build_cli_args_safe(cmd, *args, **kwargs))
        return interact(f"{self.cmd} {args}", input=stdin, stderr=stderr)


class CosmosCLI:
    "the apis to interact with wallet and blockchain"

    def __init__(
        self,
        data_dir,
        node_rpc,
        cmd,
    ):
        self.data_dir = data_dir
        self._genesis = json.loads(
            (self.data_dir / "config" / "genesis.json").read_text()
        )
        self.chain_id = self._genesis["chain_id"]
        self.node_rpc = node_rpc
        self.raw = ChainCommand(cmd)
        self.output = None
        self.error = None

    def node_id(self):
        "get tendermint node id"
        output = self.raw("tendermint", "show-node-id", home=self.data_dir)
        return output.decode().strip()

    def delete_account(self, name):
        "delete wallet account in node's keyring"
        return self.raw(
            "keys",
            "delete",
            name,
            "-y",
            "--force",
            home=self.data_dir,
            output="json",
            keyring_backend="test",
        )

    def create_account(self, name, mnemonic=None):
        "create new keypair in node's keyring"
        if mnemonic is None:
            output = self.raw(
                "keys",
                "add",
                name,
                home=self.data_dir,
                output="json",
                keyring_backend="test",
            )
        else:
            output = self.raw(
                "keys",
                "add",
                name,
                "--recover",
                home=self.data_dir,
                output="json",
                keyring_backend="test",
                stdin=mnemonic.encode() + b"\n",
            )
        return json.loads(output)

    def migrate_keystore(self):
        return self.raw("keys", "migrate", home=self.data_dir)

    def init(self, moniker):
        "the node's config is already added"
        return self.raw(
            "init",
            moniker,
            chain_id=self.chain_id,
            home=self.data_dir,
        )

    def migrate_sdk_genesis(self, version, path):
        return json.loads(self.raw("migrate", version, path))

    def migrate_cronos_genesis(self, version, path):
        return json.loads(
            self.raw(
                "tx",
                "cronos",
                "migrate",
                version,
                path,
            )
        )

    def validate_genesis(self, path):
        return self.raw("validate-genesis", path)

    def add_genesis_account(self, addr, coins, **kwargs):
        return self.raw(
            "add-genesis-account",
            addr,
            coins,
            home=self.data_dir,
            output="json",
            **kwargs,
        )

    def gentx(self, name, coins, min_self_delegation=1, pubkey=None):
        return self.raw(
            "gentx",
            name,
            coins,
            min_self_delegation=str(min_self_delegation),
            home=self.data_dir,
            chain_id=self.chain_id,
            keyring_backend="test",
            pubkey=pubkey,
        )

    def collect_gentxs(self, gentx_dir):
        return self.raw("collect-gentxs", gentx_dir, home=self.data_dir)

    def status(self):
        return json.loads(self.raw("status", node=self.node_rpc))

    def block_height(self):
        return int(self.status()["SyncInfo"]["latest_block_height"])

    def block_time(self):
        return isoparse(self.status()["SyncInfo"]["latest_block_time"])

    def balances(self, addr):
        return json.loads(
            self.raw("query", "bank", "balances", addr, home=self.data_dir)
        )["balances"]

    def balance(self, addr, denom="basetcro"):
        denoms = {coin["denom"]: int(coin["amount"]) for coin in self.balances(addr)}
        return denoms.get(denom, 0)

    def query_tx(self, tx_type, tx_value):
        tx = self.raw(
            "query",
            "tx",
            "--type",
            tx_type,
            tx_value,
            home=self.data_dir,
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return json.loads(tx)

    def query_all_txs(self, addr):
        txs = self.raw(
            "query",
            "txs-all",
            addr,
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return json.loads(txs)

    def tx_search(self, events: str):
        "/tx_search"
        return json.loads(
            self.raw("query", "txs", events=events, output="json", node=self.node_rpc)
        )

    def distribution_commission(self, addr):
        coin = json.loads(
            self.raw(
                "query",
                "distribution",
                "commission",
                addr,
                output="json",
                node=self.node_rpc,
            )
        )["commission"][0]
        return float(coin["amount"])

    def distribution_community(self):
        coin = json.loads(
            self.raw(
                "query",
                "distribution",
                "community-pool",
                output="json",
                node=self.node_rpc,
            )
        )["pool"][0]
        return float(coin["amount"])

    def distribution_reward(self, delegator_addr):
        coin = json.loads(
            self.raw(
                "query",
                "distribution",
                "rewards",
                delegator_addr,
                output="json",
                node=self.node_rpc,
            )
        )["total"][0]
        return float(coin["amount"])

    def address(self, name, bech="acc", field="address"):
        output = self.raw(
            "keys",
            "show",
            name,
            f"--{field}",
            home=self.data_dir,
            keyring_backend="test",
            bech=bech,
        )
        return output.strip().decode()

    def account(self, addr):
        return json.loads(
            self.raw(
                "query", "auth", "account", addr, output="json", node=self.node_rpc
            )
        )

    def total_supply(self):
        return json.loads(
            self.raw("query", "bank", "total", output="json", node=self.node_rpc)
        )

    def validator(self, addr):
        return json.loads(
            self.raw(
                "query",
                "staking",
                "validator",
                addr,
                output="json",
                node=self.node_rpc,
            )
        )

    def validators(self):
        return json.loads(
            self.raw(
                "query", "staking", "validators", output="json", node=self.node_rpc
            )
        )["validators"]

    def staking_params(self):
        return json.loads(
            self.raw("query", "staking", "params", output="json", node=self.node_rpc)
        )

    def staking_pool(self, bonded=True):
        return int(
            json.loads(
                self.raw("query", "staking", "pool", output="json", node=self.node_rpc)
            )["bonded_tokens" if bonded else "not_bonded_tokens"]
        )

    def transfer(self, from_, to, coins, generate_only=False, fees=None, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        return json.loads(
            self.raw(
                "tx",
                "bank",
                "send",
                from_,
                to,
                coins,
                "-y",
                "--generate-only" if generate_only else None,
                home=self.data_dir,
                fees=fees,
                **kwargs,
            )
        )

    def get_delegated_amount(self, which_addr):
        return json.loads(
            self.raw(
                "query",
                "staking",
                "delegations",
                which_addr,
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
                output="json",
            )
        )

    def delegate_amount(self, to_addr, amount, from_addr, gas_price=None):
        if gas_price is None:
            return json.loads(
                self.raw(
                    "tx",
                    "staking",
                    "delegate",
                    to_addr,
                    amount,
                    "-y",
                    home=self.data_dir,
                    from_=from_addr,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                    node=self.node_rpc,
                )
            )
        else:
            return json.loads(
                self.raw(
                    "tx",
                    "staking",
                    "delegate",
                    to_addr,
                    amount,
                    "-y",
                    home=self.data_dir,
                    from_=from_addr,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                    node=self.node_rpc,
                    gas_prices=gas_price,
                )
            )

    # to_addr: croclcl1...  , from_addr: cro1...
    def unbond_amount(self, to_addr, amount, from_addr):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "unbond",
                to_addr,
                amount,
                "-y",
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    # to_validator_addr: crocncl1...  ,  from_from_validator_addraddr: crocl1...
    def redelegate_amount(
        self, to_validator_addr, from_validator_addr, amount, from_addr
    ):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "redelegate",
                from_validator_addr,
                to_validator_addr,
                amount,
                "-y",
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    # from_delegator can be account name or address
    def withdraw_all_rewards(self, from_delegator):
        return json.loads(
            self.raw(
                "tx",
                "distribution",
                "withdraw-all-rewards",
                "-y",
                from_=from_delegator,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def make_multisig(self, name, signer1, signer2):
        self.raw(
            "keys",
            "add",
            name,
            multisig=f"{signer1},{signer2}",
            multisig_threshold="2",
            home=self.data_dir,
            keyring_backend="test",
        )

    def sign_multisig_tx(self, tx_file, multi_addr, signer_name):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer_name,
                multisig=multi_addr,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def sign_batch_multisig_tx(
        self, tx_file, multi_addr, signer_name, account_number, sequence_number
    ):
        r = self.raw(
            "tx",
            "sign-batch",
            "--offline",
            tx_file,
            account_number=account_number,
            sequence=sequence_number,
            from_=signer_name,
            multisig=multi_addr,
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return r.decode("utf-8")

    def encode_signed_tx(self, signed_tx):
        return self.raw(
            "tx",
            "encode",
            signed_tx,
        )

    def sign_single_tx(self, tx_file, signer_name):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer_name,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def combine_multisig_tx(self, tx_file, multi_name, signer1_file, signer2_file):
        return json.loads(
            self.raw(
                "tx",
                "multisign",
                tx_file,
                multi_name,
                signer1_file,
                signer2_file,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def combine_batch_multisig_tx(
        self, tx_file, multi_name, signer1_file, signer2_file
    ):
        r = self.raw(
            "tx",
            "multisign-batch",
            tx_file,
            multi_name,
            signer1_file,
            signer2_file,
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return r.decode("utf-8")

    def broadcast_tx(self, tx_file, **kwargs):
        kwargs.setdefault("broadcast_mode", "block")
        kwargs.setdefault("output", "json")
        return json.loads(
            self.raw("tx", "broadcast", tx_file, node=self.node_rpc, **kwargs)
        )

    def broadcast_tx_json(self, tx, **kwargs):
        with tempfile.NamedTemporaryFile("w") as fp:
            json.dump(tx, fp)
            fp.flush()
            return self.broadcast_tx(fp.name)

    def unjail(self, addr):
        return json.loads(
            self.raw(
                "tx",
                "slashing",
                "unjail",
                "-y",
                from_=addr,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def create_validator(
        self,
        amount,
        moniker=None,
        commission_max_change_rate="0.01",
        commission_rate="0.1",
        commission_max_rate="0.2",
        min_self_delegation="1",
        identity="",
        website="",
        security_contact="",
        details="",
    ):
        """MsgCreateValidator
        create the node with create_node before call this"""
        pubkey = (
            "'"
            + (
                self.raw(
                    "tendermint",
                    "show-validator",
                    home=self.data_dir,
                )
                .strip()
                .decode()
            )
            + "'"
        )
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "create-validator",
                "-y",
                from_=self.address("validator"),
                amount=amount,
                pubkey=pubkey,
                min_self_delegation=min_self_delegation,
                # commision
                commission_rate=commission_rate,
                commission_max_rate=commission_max_rate,
                commission_max_change_rate=commission_max_change_rate,
                # description
                moniker=moniker,
                identity=identity,
                website=website,
                security_contact=security_contact,
                details=details,
                # basic
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def edit_validator(
        self,
        commission_rate=None,
        moniker=None,
        identity=None,
        website=None,
        security_contact=None,
        details=None,
    ):
        """MsgEditValidator"""
        options = dict(
            commission_rate=commission_rate,
            # description
            moniker=moniker,
            identity=identity,
            website=website,
            security_contact=security_contact,
            details=details,
        )
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "edit-validator",
                "-y",
                from_=self.address("validator"),
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
                **{k: v for k, v in options.items() if v is not None},
            )
        )

    def gov_propose_v0_7(self, proposer, kind, proposal, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        if kind == "software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-proposal",
                    kind,
                    proposal["name"],
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    upgrade_height=proposal.get("upgrade-height"),
                    upgrade_time=proposal.get("upgrade-time"),
                    upgrade_info=proposal.get("upgrade-info"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    **kwargs,
                )
            )
        elif kind == "cancel-software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-proposal",
                    kind,
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    **kwargs,
                )
            )
        else:
            with tempfile.NamedTemporaryFile("w") as fp:
                json.dump(proposal, fp)
                fp.flush()
                return json.loads(
                    self.raw(
                        "tx",
                        "gov",
                        "submit-proposal",
                        kind,
                        fp.name,
                        "-y",
                        from_=proposer,
                        # basic
                        home=self.data_dir,
                        **kwargs,
                    )
                )

    def gov_propose_legacy(self, proposer, kind, proposal, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        if kind == "software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-legacy-proposal",
                    kind,
                    proposal["name"],
                    "-y",
                    "--no-validate",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    upgrade_height=proposal.get("upgrade-height"),
                    upgrade_time=proposal.get("upgrade-time"),
                    upgrade_info=proposal.get("upgrade-info"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    **kwargs,
                )
            )
        elif kind == "cancel-software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-legacy-proposal",
                    kind,
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    **kwargs,
                )
            )
        else:
            with tempfile.NamedTemporaryFile("w") as fp:
                json.dump(proposal, fp)
                fp.flush()
                return json.loads(
                    self.raw(
                        "tx",
                        "gov",
                        "submit-legacy-proposal",
                        kind,
                        fp.name,
                        "-y",
                        from_=proposer,
                        # basic
                        home=self.data_dir,
                        **kwargs,
                    )
                )

    def gov_vote(self, voter, proposal_id, option, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        return json.loads(
            self.raw(
                "tx",
                "gov",
                "vote",
                proposal_id,
                option,
                "-y",
                from_=voter,
                home=self.data_dir,
                **kwargs,
            )
        )

    def gov_deposit(self, depositor, proposal_id, amount):
        return json.loads(
            self.raw(
                "tx",
                "gov",
                "deposit",
                proposal_id,
                amount,
                "-y",
                from_=depositor,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def query_proposals(self, depositor=None, limit=None, status=None, voter=None):
        return json.loads(
            self.raw(
                "query",
                "gov",
                "proposals",
                depositor=depositor,
                count_total=limit,
                status=status,
                voter=voter,
                output="json",
                node=self.node_rpc,
            )
        )

    def query_proposal(self, proposal_id):
        return json.loads(
            self.raw(
                "query",
                "gov",
                "proposal",
                proposal_id,
                output="json",
                node=self.node_rpc,
            )
        )

    def query_tally(self, proposal_id):
        return json.loads(
            self.raw(
                "query",
                "gov",
                "tally",
                proposal_id,
                output="json",
                node=self.node_rpc,
            )
        )

    def ibc_transfer(
        self,
        from_,
        to,
        amount,
        channel,  # src channel
        target_version,  # chain version number of target chain
        fee,
        i=0,
    ):
        return json.loads(
            self.raw(
                "tx",
                "ibc-transfer",
                "transfer",
                "transfer",  # src port
                channel,
                to,
                amount,
                "--fees",
                fee,
                "-y",
                # FIXME https://github.com/cosmos/cosmos-sdk/issues/8059
                "--absolute-timeouts",
                from_=from_,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
                packet_timeout_height=f"{target_version}-10000000000",
                packet_timeout_timestamp=0,
            )
        )

    def export(self):
        return self.raw("export", home=self.data_dir)

    def unsaferesetall(self):
        return self.raw("tendermint", "unsafe-reset-all")

    def create_nft(self, from_addr, denomid, denomname, schema, fees):
        return json.loads(
            self.raw(
                "tx",
                "nft",
                "issue",
                denomid,
                "-y",
                fees=fees,
                name=denomname,
                schema=schema,
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def query_nft(self, denomid):
        return json.loads(
            self.raw(
                "query",
                "nft",
                "denom",
                denomid,
                output="json",
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def query_denom_by_name(self, denomname):
        return json.loads(
            self.raw(
                "query",
                "nft",
                "denom-by-name",
                denomname,
                output="json",
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def create_nft_token(self, from_addr, to_addr, denomid, tokenid, uri, fees):
        return json.loads(
            self.raw(
                "tx",
                "nft",
                "mint",
                denomid,
                tokenid,
                "-y",
                uri=uri,
                recipient=to_addr,
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def query_nft_token(self, denomid, tokenid):
        return json.loads(
            self.raw(
                "query",
                "nft",
                "token",
                denomid,
                tokenid,
                output="json",
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def burn_nft_token(self, from_addr, denomid, tokenid):
        return json.loads(
            self.raw(
                "tx",
                "nft",
                "burn",
                denomid,
                tokenid,
                "-y",
                from_=from_addr,
                keyring_backend="test",
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def edit_nft_token(self, from_addr, denomid, tokenid, newuri, newname):
        return json.loads(
            self.raw(
                "tx",
                "nft",
                "edit",
                denomid,
                tokenid,
                "-y",
                from_=from_addr,
                uri=newuri,
                name=newname,
                keyring_backend="test",
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def transfer_nft_token(self, from_addr, to_addr, denomid, tokenid):
        return json.loads(
            self.raw(
                "tx",
                "nft",
                "transfer",
                to_addr,
                denomid,
                tokenid,
                "-y",
                from_=from_addr,
                keyring_backend="test",
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def set_delegate_keys(self, val_addr, acc_addr, eth_addr, signature, **kwargs):
        """
        val_addr: cronos validator address
        acc_addr: orchestrator's cronos address
        eth_addr: orchestrator's ethereum address
        """
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        return json.loads(
            self.raw(
                "tx",
                "gravity",
                "set-delegate-keys",
                val_addr,
                acc_addr,
                eth_addr,
                signature,
                "-y",
                home=self.data_dir,
                **kwargs,
            )
        )

    def query_gravity_params(self):
        return json.loads(self.raw("query", "gravity", "params", home=self.data_dir))

    def query_signer_set_txs(self):
        return json.loads(
            self.raw("query", "gravity", "signer-set-txs", home=self.data_dir)
        )

    def query_signer_set_tx(self, nonce):
        return json.loads(
            self.raw(
                "query", "gravity", "signer-set-tx", str(nonce), home=self.data_dir
            )
        )

    def query_latest_signer_set_tx(self):
        return json.loads(
            self.raw("query", "gravity", "latest-signer-set-tx", home=self.data_dir)
        )

    def send_to_ethereum(self, receiver, coins, fee, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        return json.loads(
            self.raw(
                "tx",
                "gravity",
                "send-to-ethereum",
                receiver,
                coins,
                fee,
                "-y",
                home=self.data_dir,
                **kwargs,
            )
        )

    def query_contract_by_denom(self, denom: str):
        "query contract by denom"
        return json.loads(
            self.raw(
                "query",
                "cronos",
                "contract-by-denom",
                denom,
                home=self.data_dir,
            )
        )

    def query_denom_by_contract(self, contract: str):
        "query denom by contract"
        return json.loads(
            self.raw(
                "query",
                "cronos",
                "denom-by-contract",
                contract,
                home=self.data_dir,
            )
        )

    def gov_propose_token_mapping_change(
        self, denom, contract, symbol, decimal, **kwargs
    ):
        default_kwargs = {
            "gas_prices": DEFAULT_GAS_PRICE,
            "gas": "auto",
            "gas_adjustment": "1.5",
        }
        return json.loads(
            self.raw(
                "tx",
                "gov",
                "submit-legacy-proposal",
                "token-mapping-change",
                denom,
                contract,
                "--symbol",
                symbol,
                "--decimals",
                decimal,
                "-y",
                home=self.data_dir,
                stderr=subprocess.DEVNULL,
                **(default_kwargs | kwargs),
            )
        )

    def gov_propose_update_client_legacy(self, proposal, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        kwargs.setdefault("gas", 600000)
        return json.loads(
            self.raw(
                "tx",
                "gov",
                "submit-legacy-proposal",
                "update-client",
                proposal.get("subject_client_id"),
                proposal.get("substitute_client_id"),
                "-y",
                from_=proposal.get("from"),
                keyring_backend="test",
                # content
                title=proposal.get("title"),
                description=proposal.get("description"),
                deposit=proposal.get("deposit"),
                chain_id=self.chain_id,
                home=self.data_dir,
                stderr=subprocess.DEVNULL,
                **kwargs,
            )
        )

    def update_token_mapping(self, denom, contract, symbol, decimals, **kwargs):
        kwargs.setdefault("gas_prices", DEFAULT_GAS_PRICE)
        return json.loads(
            self.raw(
                "tx",
                "cronos",
                "update-token-mapping",
                denom,
                contract,
                "--symbol",
                symbol,
                "--decimals",
                decimals,
                "-y",
                home=self.data_dir,
                **kwargs,
            )
        )

    def build_evm_tx(self, raw_tx: str, **kwargs):
        return json.loads(
            self.raw(
                "tx",
                "evm",
                "raw",
                raw_tx,
                "-y",
                "--generate-only",
                home=self.data_dir,
                **kwargs,
            )
        )

    def transfer_tokens(self, from_, to, amount, **kwargs):
        default_kwargs = {
            "gas": "auto",
            "gas_adjustment": "1.5",
        }
        return json.loads(
            self.raw(
                "tx",
                "cronos",
                "transfer-tokens",
                from_,
                to,
                amount,
                "-y",
                home=self.data_dir,
                stderr=subprocess.DEVNULL,
                **(default_kwargs | kwargs),
            )
        )

    def ica_register_account(self, connid, **kwargs):
        "execute on host chain to attach an account to the connection"
        default_kwargs = {
            "home": self.data_dir,
            "node": self.node_rpc,
            "chain_id": self.chain_id,
            "keyring_backend": "test",
        }
        return json.loads(
            self.raw(
                "tx",
                "icactl",
                "register-account",
                connid,
                "-y",
                **(default_kwargs | kwargs),
            )
        )

    def ica_submit_tx(self, connid, tx, **kwargs):
        default_kwargs = {
            "home": self.data_dir,
            "node": self.node_rpc,
            "chain_id": self.chain_id,
            "keyring_backend": "test",
        }
        return json.loads(
            self.raw(
                "tx",
                "icactl",
                "submit-tx",
                connid,
                tx,
                "-y",
                **(default_kwargs | kwargs),
            )
        )

    def ica_query_account(self, connid, owner, **kwargs):
        default_kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "icactl",
                "interchain-account-address",
                connid,
                owner,
                **(default_kwargs | kwargs),
            )
        )

    def ibc_query_channels(self, connid, **kwargs):
        default_kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "ibc",
                "channel",
                "connections",
                connid,
                **(default_kwargs | kwargs),
            )
        )

    def ibc_query_ack(self, port_id, channel_id, packet_seq, **kwargs):
        default_kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "ibc",
                "channel",
                "packet-ack",
                port_id,
                channel_id,
                packet_seq,
                **(default_kwargs | kwargs),
            )
        )

    def query_icactl_params(self, **kwargs):
        default_kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "icactl",
                "params",
                **(default_kwargs | kwargs),
            )
        )

    def query_icacontroller_params(self, **kwargs):
        default_kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "interchain-accounts",
                "controller",
                "params",
                **(default_kwargs | kwargs),
            )
        )

    def query_gravity_contract_by_denom(self, denom: str):
        "query CosmosERC20 contract address by denom"
        return json.loads(
            self.raw(
                "query",
                "gravity",
                "denom-to-erc20",
                denom,
                home=self.data_dir,
            )
        )

    def create_vesting_account(self, to_address, amount, end_time, **kwargs):
        "create vesting account"
        default_kwargs = {
            "home": self.data_dir,
            "node": self.node_rpc,
            "chain_id": self.chain_id,
            "keyring_backend": "test",
        }
        return json.loads(
            self.raw(
                "tx",
                "vesting",
                "create-vesting-account",
                to_address,
                amount,
                end_time,
                "-y",
                **(default_kwargs | kwargs),
            )
        )

    def register_counterparty_payee(
        self, port_id, channel_id, relayer, counterparty_payee, **kwargs
    ):
        default_kwargs = {
            "home": self.data_dir,
        }
        return json.loads(
            self.raw(
                "tx",
                "ibc-fee",
                "register-counterparty-payee",
                port_id,
                channel_id,
                relayer,
                counterparty_payee,
                "-y",
                **(default_kwargs | kwargs),
            )
        )

    def register_payee(self, port_id, channel_id, relayer, payee, **kwargs):
        default_kwargs = {
            "home": self.data_dir,
        }
        return json.loads(
            self.raw(
                "tx",
                "ibc-fee",
                "register-payee",
                port_id,
                channel_id,
                relayer,
                payee,
                "-y",
                **(default_kwargs | kwargs),
            )
        )

    def pay_packet_fee(self, port_id, channel_id, packet_seq, **kwargs):
        default_kwargs = {
            "home": self.data_dir,
        }
        return json.loads(
            self.raw(
                "tx",
                "ibc-fee",
                "pay-packet-fee",
                port_id,
                channel_id,
                str(packet_seq),
                "-y",
                **(default_kwargs | kwargs),
            )
        )

    def evm_params(self, **kwargs):
        default_kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "evm",
                "params",
                **(default_kwargs | kwargs),
            )
        )

    def changeset_dump(self, changeset_dir, **kwargs):
        default_kwargs = {
            "home": self.data_dir,
        }
        return self.raw(
            "changeset", "dump", changeset_dir, **(default_kwargs | kwargs)
        ).decode()

    def changeset_verify(self, changeset_dir, **kwargs):
        output = self.raw("changeset", "verify", changeset_dir, **kwargs).decode()
        hash, commit_info = output.split("\n")
        return binascii.unhexlify(hash), json.loads(commit_info)

    def changeset_restore_app_db(self, snapshot_dir, app_db, **kwargs):
        return self.raw(
            "changeset", "restore-app-db", snapshot_dir, app_db, **kwargs
        ).decode()

    def changeset_build_versiondb_sst(self, changeset_dir, sst_dir, **kwargs):
        return self.raw(
            "changeset", "build-versiondb-sst", changeset_dir, sst_dir, **kwargs
        ).decode()

    def changeset_ingest_versiondb_sst(self, versiondb_dir, sst_dir, **kwargs):
        sst_files = [os.path.join(sst_dir, name) for name in os.listdir(sst_dir)]
        return self.raw(
            "changeset",
            "ingest-versiondb-sst",
            versiondb_dir,
            *sst_files,
            "--move-files",
            **kwargs,
        ).decode()
