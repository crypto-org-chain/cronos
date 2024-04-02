// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

contract Random {
    function randomTokenId() public returns (uint256) {
        return uint256(keccak256(abi.encodePacked(block.prevrandao)));
    }
}