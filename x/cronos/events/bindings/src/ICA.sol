// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IICAModule {
    event SubmitMsgsResult(string indexed packetSrcChannel, uint64 seq);
    function registerAccount(string calldata connectionID, string calldata version, int32 ordering) external payable returns (bool);
    function queryAccount(string calldata connectionID, address addr) external view returns (string memory);
    function submitMsgs(string calldata connectionID, bytes calldata data, uint256 timeout) external payable returns (uint64);
}
