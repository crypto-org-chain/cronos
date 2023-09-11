// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

contract TestICA {
    address constant icaContract = 0x0000000000000000000000000000000000000066;

    function nativeRegister(string memory connectionID) public {
        (bool result,) = icaContract.call(abi.encodeWithSignature(
            "registerAccount(string,address)",
            connectionID, msg.sender
        ));
        require(result, "native call failed");
    }

    function nativeQueryAccount(string memory connectionID, address addr) public returns (bytes memory) {
        (bool result, bytes memory data) = icaContract.call(abi.encodeWithSignature(
            "queryAccount(string,address)",
            connectionID, addr
        ));
        require(result, "native call failed");
        return data;
    }
}