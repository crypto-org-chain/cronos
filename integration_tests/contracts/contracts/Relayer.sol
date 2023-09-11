// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {Cosmos} from "./CosmosTypes.sol";

interface IRelayerModule {
    // Client
    event CreateClient(
        string indexed clientId,
        string indexed clientType
    );
    event UpdateClient(
        string indexed clientId,
        string indexed clientType
    );
    event UpgradeClient(
        string indexed clientId,
        string indexed clientType
    );
    event SubmitMisbehaviour(
        string indexed subjectId,
        string indexed clientType
    );
    // Connection
    event ConnectionOpenInit(
        string indexed connectionId,
        string indexed clientId,
        string indexed counterpartyClientId,
        string counterpartyConnectionId
    );
    event ConnectionOpenTry(
        string indexed connectionId,
        string indexed clientId,
        string indexed counterpartyClientId,
        string counterpartyConnectionId
    );
    event ConnectionOpenAck(
        string indexed connectionId,
        string indexed clientId,
        string indexed counterpartyClientId,
        string counterpartyConnectionId
    );
    event ConnectionOpenConfirm(
        string indexed connectionId,
        string indexed clientId,
        string indexed counterpartyClientId,
        string counterpartyConnectionId
    );
    // Channel
    event ChannelOpenInit(
        string indexed portId,
        string indexed channelId,
        string indexed counterpartyPortId,
        string counterpartyChannelId,
        string connectionId,
        string version
    );
    event ChannelOpenTry(
        string indexed portId,
        string indexed channelId,
        string indexed counterpartyPortId,
        string counterpartyChannelId,
        string connectionId,
        string version
    );
    event ChannelOpenAck(
        string indexed portId,
        string indexed channelId,
        string indexed counterpartyPortId,
        string counterpartyChannelId,
        string connectionId
    );
    event ChannelOpenConfirm(
        string indexed portId,
        string indexed channelId,
        string indexed counterpartyPortId,
        string counterpartyChannelId,
        string connectionId
    );
    event ChannelCloseInit(
        string indexed portId,
        string indexed channelId,
        string indexed counterpartyPortId,
        string counterpartyChannelId,
        string connectionId
    );
    event ChannelCloseConfirm(
        string indexed portId,
        string indexed channelId,
        string indexed counterpartyPortId,
        string counterpartyChannelId,
        string connectionId
    );
    struct PacketData {
        address receiver;
        string sender;
        Cosmos.Coin[] amount;
    }
    event RecvPacket(PacketData packetData);
    event WriteAcknowledgement(
        string packetConnection,
        PacketData packetData
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
    event Message();
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
