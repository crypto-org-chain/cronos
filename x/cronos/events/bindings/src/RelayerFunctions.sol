// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IRelayerFunctions {
    function createClient(bytes calldata data) external payable returns (bytes calldata);
    function updateClient(bytes calldata data) external payable returns (bytes calldata);
    function upgradeClient(bytes calldata data) external payable returns (bytes calldata);
    function submitMisbehaviour(bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenInit(bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenTry(bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenAck(bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenConfirm(bytes calldata data) external payable returns (bytes calldata);
    function channelOpenInit(bytes calldata data) external payable returns (bytes calldata);
    function channelOpenTry(bytes calldata data) external payable returns (bytes calldata);
    function channelOpenAck(bytes calldata data) external payable returns (bytes calldata);
    function channelOpenConfirm(bytes calldata data) external payable returns (bytes calldata);
    function channelCloseInit(bytes calldata data) external payable returns (bytes calldata);
    function channelCloseConfirm(bytes calldata data) external payable returns (bytes calldata);
    function recvPacket(bytes calldata data) external payable returns (bytes calldata);
    function acknowledgement(bytes calldata data) external payable returns (bytes calldata);
    function timeout(bytes calldata data) external payable returns (bytes calldata);
    function timeoutOnClose(bytes calldata data) external payable returns (bytes calldata);
    function registerPayee(string calldata portID, string calldata channelID, address relayerAddr) external payable returns (bool);
    function registerCounterpartyPayee(string calldata portID, string calldata channelID, address relayerAddr) external payable returns (bool);
}
