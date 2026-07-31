from dataclasses import dataclass

ICA_PRECOMPILE_ADDRESS = "0x0000000000000000000000000000000000000066"

ICA_ABI = [
    {
        "inputs": [
            {"internalType": "string", "name": "connectionID", "type": "string"},
            {"internalType": "string", "name": "version", "type": "string"},
            {"internalType": "int32", "name": "ordering", "type": "int32"},
        ],
        "name": "registerAccount",
        "outputs": [{"internalType": "bool", "name": "", "type": "bool"}],
        "stateMutability": "payable",
        "type": "function",
    },
]

UNKNOWN_CONNECTION_ID = "connection-999999999"


@dataclass
class IcaRejectionResult:
    rejected: bool
    error: str | None = None


def register_ica_with_unknown_connection(w3, account) -> IcaRejectionResult:
    """Call the ICA controller precompile's registerAccount with a
    connection-id that doesn't exist. x/cronos/keeper/precompiles/ica.go
    forwards the call into icacontrollerkeeper.RegisterInterchainAccount,
    which builds a MsgChannelOpenInit and dispatches it through the msg
    router; ibc-go's 03-connection keeper returns ErrConnectionNotFound for
    an unknown connection-id, and that error propagates back out of the
    precompile's Run as an EVM revert, leaving receipt.status == 0."""
    ica = w3.eth.contract(address=ICA_PRECOMPILE_ADDRESS, abi=ICA_ABI)
    try:
        tx = ica.functions.registerAccount(UNKNOWN_CONNECTION_ID, "", 0).build_transaction(
            {
                "chainId": w3.eth.chain_id,
                "from": account.address,
                "nonce": w3.eth.get_transaction_count(account.address, "pending"),
                "gasPrice": w3.eth.gas_price,
                # Explicit gas skips web3.py's implicit eth_estimateGas, which
                # would raise on the revert before we can read receipt.status.
                "gas": 500_000,
            }
        )
        signed = account.sign_transaction(tx)
        tx_hash = w3.eth.send_raw_transaction(signed.raw_transaction)
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash)
    except Exception as exc:  # noqa: BLE001
        return IcaRejectionResult(rejected=False, error=str(exc))

    return IcaRejectionResult(rejected=receipt.status == 0)
