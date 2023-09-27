// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

contract ICACallback {
    // sha256('cronos-evm')[:20]
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
    uint64 lastAckSeq;
    bytes lastAck;
    mapping (uint64 => bytes) public acknowledgement;
    event OnPacketResult(uint64 seq, bytes ack);

    function onPacketResultCallback(uint64 seq, bytes calldata ack) external payable returns (bool) {
        // require(msg.sender == module_address);
        lastAckSeq = seq;
        lastAck = ack;
        acknowledgement[seq] = ack;
        emit OnPacketResult(seq, ack);
        return true;
    }
}
