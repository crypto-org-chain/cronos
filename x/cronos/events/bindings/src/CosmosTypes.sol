// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

library Cosmos {
    struct Coin {
        uint256 amount;
        string denom;
    }
    struct Hop {
        string portId;
        string channelId;
    }
    struct Denom {
        string base;
        Hop[] trace;
    }
    struct Token {
        uint256 amount;
        Denom denom;
    }
}

contract CosmosTypes {
    function coin(Cosmos.Coin calldata) public pure {}
    function token(Cosmos.Token calldata) public pure {}
}
