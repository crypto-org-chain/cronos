// SPDX-License-Identifier: MIT
pragma solidity >0.6.6;

contract TestICA {
    address constant icaContract = 0x0000000000000000000000000000000000000066;

    function encodeRegister(string memory connectionID) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "registerAccount(string,address,string)",
            connectionID, msg.sender, ""
        );
    }

    function callRegister(string memory connectionID) public {
        (bool result,) = icaContract.call(encodeRegister(connectionID));
        require(result, "call failed");
    }

    function delegateRegister(string memory connectionID) public {
        (bool result,) = icaContract.delegatecall(encodeRegister(connectionID));
        require(result, "call failed");
    }

    function staticRegister(string memory connectionID) public {
        (bool result,) = icaContract.staticcall(encodeRegister(connectionID));
        require(result, "call failed");
    }

    function encodeQueryAccount(string memory connectionID, address addr) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "queryAccount(string,address)",
            connectionID, addr
        );
    }

    function callQueryAccount(string memory connectionID, address addr) public returns (string memory) {
        (bool result, bytes memory data) = icaContract.call(encodeQueryAccount(connectionID, addr));
        require(result, "call failed");
        return abi.decode(data, (string));
    }

    function delegateQueryAccount(string memory connectionID, address addr) public returns (string memory) {
        (bool result, bytes memory data) = icaContract.delegatecall(encodeQueryAccount(connectionID, addr));
        require(result, "call failed");
        return abi.decode(data, (string));
    }

    function staticQueryAccount(string memory connectionID, address addr) public returns (string memory) {
        (bool result, bytes memory data) = icaContract.staticcall(encodeQueryAccount(connectionID, addr));
        require(result, "call failed");
        return abi.decode(data, (string));
    }

    function encodeSubmitMsgs(string memory connectionID, string memory data) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "submitMsgs(string,address,string,uint256)",
            connectionID, msg.sender, data, 300000000000
        );
    }

    function callSubmitMsgs(string memory connectionID, string memory data) public {
        (bool result,) = icaContract.call(encodeSubmitMsgs(connectionID, data));
        require(result, "call failed");
    }

    function delegateSubmitMsgs(string memory connectionID, string memory data) public {
        (bool result,) = icaContract.delegatecall(encodeSubmitMsgs(connectionID, data));
        require(result, "call failed");
    }

    function staticSubmitMsgs(string memory connectionID, string memory data) public {
        (bool result,) = icaContract.staticcall(encodeSubmitMsgs(connectionID, data));
        require(result, "call failed");
    }
}