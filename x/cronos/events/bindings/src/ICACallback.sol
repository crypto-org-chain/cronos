// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IICACallback {
    function onPacketResultCallback(uint64 seq, bool ack) external payable returns (bool);
}
