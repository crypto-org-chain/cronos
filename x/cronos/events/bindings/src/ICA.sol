// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IICAModule {
    // ICS27 Interchain Accounts events
    event Ics27Packet(string indexed controllerChannelId);
}
