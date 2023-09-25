// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {Cosmos} from "./CosmosTypes.sol";

interface IRelayerModule {
    struct PacketData {
        address receiver;
        string sender;
        Cosmos.Coin[] amount;
    }
    event RecvPacket(PacketData packetDataHex);
    event WriteAcknowledgement(
        string packetConnection,
        PacketData packetDataHex
    );
    event AcknowledgePacket(
        string indexed packetSrcPort,
        string indexed packetSrcChannel,
        string indexed packetDstPort,
        string packetDstChannel,
        string packetChannelOrdering,
        string packetConnection
    );
    event TimeoutPacket(
        string indexed packetSrcPort,
        string indexed packetSrcChannel,
        string indexed packetDstPort,
        string packetDstChannel,
        string packetChannelOrdering
    );
    // IBC transfer
    event Timeout(
        address indexed refundReceiver,
        string indexed refundDenom,
        uint256 amount
    );
    event FungibleTokenPacket(
        address indexed receiver,
        address indexed sender,
        string indexed denom,
        uint256 amount
    );
    event IbcTransfer(
        address indexed sender,
        address indexed receiver,
        string indexed denom,
        uint256 amount
    );
    event ChannelClosed();
    event DenominationTrace(string indexed denom);
    // 29-fee
    event DistributeFee(
        address indexed receiver,
        string indexed fee
    );
    // Bank
    event Transfer(
        address indexed recipient,
        address indexed sender,
        Cosmos.Coin[] amount
    );
    event CoinReceived(address indexed receiver, Cosmos.Coin[] amount);
    event Coinbase(address indexed minter, Cosmos.Coin[] amount);
    event CoinSpent(address indexed spender, Cosmos.Coin[] amount);
    event Burn(address indexed burner, Cosmos.Coin[] amount);
}
