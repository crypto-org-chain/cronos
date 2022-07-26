pragma solidity ^0.6.6;

contract CronosGravityCancellation {

    event __CronosCancelSendToChain(address sender, uint256 id);

    // Cancel a send to chain transaction considering if it hasnt been batched yet.
    function cancelTransaction(uint256 id) public {
        emit __CronosCancelSendToChain(msg.sender, id);
    }
}
