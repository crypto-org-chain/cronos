// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {IICAModule} from "./src/ICA.sol";

contract TestICA {
    address constant icaContract = 0x0000000000000000000000000000000000000066;
    IICAModule ica = IICAModule(icaContract);
    address account;
    // sha256('cronos-evm')[:20]
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
    uint64 lastSeq;
    enum Status {
        PENDING,
        SUCCESS,
        FAIL
    }
    mapping (string => mapping (uint64 => Status)) public statusMap;
    event OnPacketResult(string indexed packetSrcChannel, uint64 seq, Status status);

    function encodeRegister(string memory connectionID, string memory version, int32 ordering) internal view returns (bytes memory) {
        return abi.encodeWithSignature(
            "registerAccount(string,string,int32)",
            connectionID, version, ordering
        );
    }

    function callRegister(string memory connectionID, string memory version, int32 ordering) public returns (bool) {
        require(account == address(0) || account == msg.sender, "register fail");
        bool result = ica.registerAccount(connectionID, version, ordering);
        require(result, "call failed");
        account = msg.sender;
    }

    function getAccount() public view returns (address) {
        return account;
    }

    function delegateRegister(string memory connectionID, string memory version, int32 ordering) public returns (bool) {
        (bool result,) = icaContract.delegatecall(encodeRegister(connectionID, version, ordering));
        require(result, "call failed");
        return true;
    }

    function staticRegister(string memory connectionID, string memory version, int32 ordering) public returns (bool) {
        (bool result,) = icaContract.staticcall(encodeRegister(connectionID, version, ordering));
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
            connectionID, data, timeout
        );
    }

    function callSubmitMsgs(string memory connectionID, string calldata packetSrcChannel, bytes memory data, uint256 timeout) public returns (uint64) {
        require(account == msg.sender, "not authorized");
        lastSeq = ica.submitMsgs(connectionID, data, timeout);
        statusMap[packetSrcChannel][lastSeq] = Status.PENDING;
        return lastSeq;
    }

    function delegateSubmitMsgs(string memory connectionID, bytes memory data, uint256 timeout) public returns (uint64) {
        (bool result, bytes memory data) = icaContract.delegatecall(encodeSubmitMsgs(connectionID, data, timeout));
        require(result, "call failed");
        lastSeq = abi.decode(data, (uint64));
        return lastSeq;
    }

    function staticSubmitMsgs(string memory connectionID, bytes memory data, uint256 timeout) public returns (uint64) {
        (bool result, bytes memory data) = icaContract.staticcall(encodeSubmitMsgs(connectionID, data, timeout));
        require(result, "call failed");
        lastSeq = abi.decode(data, (uint64));
        return lastSeq;
    }

    function getLastSeq() public view returns (uint256) {
        return lastSeq;
    }

    function getStatus(string calldata packetSrcChannel, uint64 seq) public view returns (Status) {
        return statusMap[packetSrcChannel][seq];
    }

    function onPacketResultCallback(string calldata packetSrcChannel, uint64 seq, bool ack) external payable returns (bool) {
        // To prevent called by arbitrary user
        require(msg.sender == module_address);
        Status currentStatus = statusMap[packetSrcChannel][seq];
        if (currentStatus != Status.PENDING) {
            return true;
        }
        delete statusMap[packetSrcChannel][seq];
        Status status = Status.FAIL;
        if (ack) {
            status = Status.SUCCESS;
        }
        statusMap[packetSrcChannel][seq] = status;
        emit OnPacketResult(packetSrcChannel, seq, status);
        return true;
    }
}
