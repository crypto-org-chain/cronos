// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IICAModule {
    event SubmitMsgsResult(uint64 seq);
    event DestinationCallback(
        string module,
        string callbackType,
        string callbackAddress,
        string callbackResult,
        string callbackError,
        string callbackExecGasLimit,
        string callbackCommitGasLimit,
        string packetDestPort,
        string packetDestChannel,
        string packetSequence
    );
    event SourceCallback(
        string module,
        string callbackType,
        string callbackAddress,
        string callbackResult,
        string callbackExecGasLimit,
        string callbackCommitGasLimit,
        string packetSrcPort,
        string packetSrcChannel,
        string packetSequence
    );
    function registerAccount(string calldata connectionID, string calldata version) external payable returns (bool);
    function queryAccount(string calldata connectionID, address addr) external view returns (string memory);
    function submitMsgs(string calldata connectionID, bytes calldata data, uint256 timeout) external payable returns (uint64);
    function onAcknowledgementPacketCallback(uint64 seq, string calldata packetSenderAddress) external payable returns (bool);
    function onTimeoutPacketCallback(uint64 seq, string calldata packetSenderAddress) external payable returns (bool);
}
