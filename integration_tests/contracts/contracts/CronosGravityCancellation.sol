pragma solidity ^0.6.6;

contract CronosGravityCancellation {

    event __CronosCancelSendToEthereum(uint256 id);

    // Cancel a send to ethereum transaction considering if it hasnt been batched yet.
    function cancelTransaction(uint256 id) public {
        emit __CronosCancelSendToEthereum(id);
    }
}
