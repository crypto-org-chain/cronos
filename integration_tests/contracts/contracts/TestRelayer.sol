// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {IRelayerFunctions} from "./src/RelayerFunctions.sol";

contract TestRelayer {
    address constant relayerContract = 0x0000000000000000000000000000000000000065;
    IRelayerFunctions relayer = IRelayerFunctions(relayerContract);
    address payee;
    address counterpartyPayee;

    function batchCall(bytes[] memory payloads) public {
        for (uint256 i = 0; i < payloads.length; i++) {
            (bool success,) = relayerContract.call(payloads[i]);
            require(success);
        }
    }

    function callRegisterPayee(string calldata portID, string calldata channelID, address payeeAddr) public returns (bool) {
        require(payee == address(0) || payee == msg.sender, "register fail");
        bool result = relayer.registerPayee(portID, channelID, payeeAddr);
        require(result, "call failed");
        payee = msg.sender;
    }

    function callRegisterCounterpartyPayee(string calldata portID, string calldata channelID, string calldata counterpartyPayeeAddr) public returns (bool) {
        require(counterpartyPayee == address(0) || counterpartyPayee == msg.sender, "register fail");
        bool result = relayer.registerCounterpartyPayee(portID, channelID, counterpartyPayeeAddr);
        require(result, "call failed");
        counterpartyPayee = msg.sender;
    }
}
