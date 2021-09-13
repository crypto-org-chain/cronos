pragma solidity ^0.6.6;

contract CroBridge {

    event __CronosSendCroToIbc(string recipient, uint256 amount);

    // Pay the contract a certain CRO amount and trigger a CRO transfer
    // from the contract to recipient through IBC
    function send_cro_to_crypto_org(string memory recipient) public payable {
        emit __CronosSendCroToIbc(recipient, msg.value);
    }
}
