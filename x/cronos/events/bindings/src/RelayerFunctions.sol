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
    function updateClientAndConnectionOpenInit(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndConnectionOpenTry(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndConnectionOpenAck(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndConnectionOpenConfirm(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndChannelOpenInit(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndChannelOpenTry(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndChannelOpenAck(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndChannelOpenConfirm(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndRecvPacket(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndAcknowledgement(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndTimeout(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndChannelCloseInit(bytes calldata data1, bytes calldata data2) external payable returns (bool);
    function updateClientAndChannelCloseConfirm(bytes calldata data1, bytes calldata data2) external payable returns (bool);
}
