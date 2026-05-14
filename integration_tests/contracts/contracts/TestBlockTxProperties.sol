pragma solidity 0.8.21;

contract TestBlockTxProperties {
    event TxDetailsEvent(
        address indexed origin,
        address indexed sender,
        uint value,
        bytes data,
        uint256 price,
        uint gas,
        bytes4 sig
    );

    function emitTxDetails() public payable {
        emit TxDetailsEvent(tx.origin, msg.sender, msg.value, msg.data, tx.gasprice, gasleft(), msg.sig);
    }
}
