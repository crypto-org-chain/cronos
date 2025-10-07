// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

contract TestEip2935 {
    // helper that returns blockhash opcode result (to demonstrate opcode unchanged)
    function blockhashOpcode(uint256 blk) public view returns (bytes32) {
        return blockhash(blk);
    }
}