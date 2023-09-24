// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IICAModule {
    event SubmitMsgsResult(uint64 seq);
    function registerAccount(string calldata connectionID, string calldata version) external payable returns (bool);
    function queryAccount(string calldata connectionID, address addr) external view returns (string memory);
    function submitMsgs(string calldata connectionID, string calldata data, uint256 timeout) external payable returns (uint64);
}
