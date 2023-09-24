// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {IICAModule} from "./src/ICA.sol";

contract TestICA {
    address constant icaContract = 0x0000000000000000000000000000000000000066;
    IICAModule ica = IICAModule(icaContract);

    function encodeRegister(string memory connectionID) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "registerAccount(string,string)",
            connectionID, msg.sender, ""
        );
    }

    function callRegister(string memory connectionID) public returns (bool) {
        return ica.registerAccount(connectionID, "");
    }

    function delegateRegister(string memory connectionID) public returns (bool) {
        (bool result,) = icaContract.delegatecall(encodeRegister(connectionID));
        require(result, "call failed");
        return true;
    }

    function staticRegister(string memory connectionID) public returns (bool) {
        (bool result,) = icaContract.staticcall(encodeRegister(connectionID));
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

    function encodeSubmitMsgs(string memory connectionID, string memory data) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "submitMsgs(string,string,uint256)",
            connectionID, msg.sender, data, 300000000000
        );
    }

    function callSubmitMsgs(string memory connectionID, string memory data) public returns (uint64) {
        return ica.submitMsgs(connectionID, data, 300000000000);
    }

    function delegateSubmitMsgs(string memory connectionID, string memory data) public returns (uint64) {
        (bool result, bytes memory data) = icaContract.delegatecall(encodeSubmitMsgs(connectionID, data));
        require(result, "call failed");
        return abi.decode(data, (uint64));
    }

    function staticSubmitMsgs(string memory connectionID, string memory data) public returns (uint64) {
        (bool result, bytes memory data) = icaContract.staticcall(encodeSubmitMsgs(connectionID, data));
        require(result, "call failed");
        return abi.decode(data, (uint64));
    }
}