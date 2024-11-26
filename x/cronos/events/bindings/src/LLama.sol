// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface ILLamaModule {
    function inference(string calldata prompt, uint256 temperature, uint256 seed, uint256 steps) external payable returns (string memory result);
}
