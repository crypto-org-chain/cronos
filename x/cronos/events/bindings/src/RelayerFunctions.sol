// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IRelayerFunctions {
    function createClient(address signer, bytes calldata data) external payable returns (bytes calldata);
    function updateClient(address signer, bytes calldata data) external payable returns (bytes calldata);
    function upgradeClient(address signer, bytes calldata data) external payable returns (bytes calldata);
    function submitMisbehaviour(address signer, bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenInit(address signer, bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenTry(address signer, bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenAck(address signer, bytes calldata data) external payable returns (bytes calldata);
    function connectionOpenConfirm(address signer, bytes calldata data) external payable returns (bytes calldata);
    function channelOpenInit(address signer, bytes calldata data) external payable returns (bytes calldata);
    function channelOpenTry(address signer, bytes calldata data) external payable returns (bytes calldata);
    function channelOpenAck(address signer, bytes calldata data) external payable returns (bytes calldata);
    function channelOpenConfirm(address signer, bytes calldata data) external payable returns (bytes calldata);
    function channelCloseInit(address signer, bytes calldata data) external payable returns (bytes calldata);
    function channelCloseConfirm(address signer, bytes calldata data) external payable returns (bytes calldata);
    function recvPacket(address signer, bytes calldata data) external payable returns (bytes calldata);
    function acknowledgement(address signer, bytes calldata data) external payable returns (bytes calldata);
    function timeout(address signer, bytes calldata data) external payable returns (bytes calldata);
    function timeoutOnClose(address signer, bytes calldata data) external payable returns (bytes calldata);
}
