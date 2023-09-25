// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {IICAModule} from "./src/ICA.sol";

contract TestICA {
    address constant icaContract = 0x0000000000000000000000000000000000000066;
    IICAModule ica = IICAModule(icaContract);
    address account;
    uint64 lastAckSeq;

    function encodeRegister(string memory connectionID, string memory version) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "registerAccount(string,string)",
            connectionID, msg.sender, version
        );
    }

    function callRegister(string memory connectionID, string memory version) public returns (bool) {
        require(account == address(0) || account == msg.sender, "register fail");
        bool result = ica.registerAccount(connectionID, version);
        require(result, "call failed");
        account = msg.sender;
    }

    function getAccount() public view returns (address) {
        return account;
    }

    function delegateRegister(string memory connectionID, string memory version) public returns (bool) {
        (bool result,) = icaContract.delegatecall(encodeRegister(connectionID, version));
        require(result, "call failed");
        return true;
    }

    function staticRegister(string memory connectionID, string memory version) public returns (bool) {
        (bool result,) = icaContract.staticcall(encodeRegister(connectionID, version));
        require(result, "call failed");
        return true;
    }

    function encodeQueryAccount(string memory connectionID, address addr) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "queryAccount(string,address)",
            connectionID, addr
        );
    }

    function callQueryAccount(string memory connectionID, address addr) public returns (string memory) {
        return ica.queryAccount(connectionID, addr);
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

    function encodeSubmitMsgs(string memory connectionID, bytes memory data, uint256 timeout) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "submitMsgs(string,bytes,uint256)",
            connectionID, msg.sender, data, timeout
        );
    }

    function callSubmitMsgs(string memory connectionID, bytes memory data, uint256 timeout) public returns (uint64) {
        require(account == msg.sender, "not authorized");
        lastAckSeq = ica.submitMsgs(connectionID, data, timeout);
        return lastAckSeq;
    }

    function delegateSubmitMsgs(string memory connectionID, bytes memory data, uint256 timeout) public returns (uint64) {
        (bool result, bytes memory data) = icaContract.delegatecall(encodeSubmitMsgs(connectionID, data, timeout));
        require(result, "call failed");
        lastAckSeq = abi.decode(data, (uint64));
        return lastAckSeq;
    }

    function staticSubmitMsgs(string memory connectionID, bytes memory data, uint256 timeout) public returns (uint64) {
        (bool result, bytes memory data) = icaContract.staticcall(encodeSubmitMsgs(connectionID, data, timeout));
        require(result, "call failed");
        lastAckSeq = abi.decode(data, (uint64));
        return lastAckSeq;
    }

    function getLastAckSeq() public view returns (uint256) {
        return lastAckSeq;
    }
}