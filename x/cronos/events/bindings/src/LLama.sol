// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface ILLamaModule {
    function run(string calldata prompt, uint256 seed) external payable returns (uint64);
}
