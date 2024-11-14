pragma solidity ^0.6.6;

contract CronosGravityCancellation {

    event __CronosCancelSendToEvmChain(address indexed sender, uint256 id);

    // Cancel a send to chain transaction considering if it hasn't been batched yet.
    function cancelTransaction(uint256 id) public {
        emit __CronosCancelSendToEvmChain(msg.sender, id);
    }
}
