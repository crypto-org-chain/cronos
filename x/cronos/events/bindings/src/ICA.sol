// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

interface IICAModule {
    event RegisterAccountResult(string channelId, string portId);
    event SubmitMsgsResult(string seq);
}
