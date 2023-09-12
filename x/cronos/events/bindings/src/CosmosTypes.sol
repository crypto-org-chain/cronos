// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

library Cosmos {
    struct Coin {
        uint256 amount;
        string denom;
    }
}

contract CosmosTypes {
    function coin(Cosmos.Coin calldata) public pure {}
}
