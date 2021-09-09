pragma solidity ^0.6.11;

import "ds-token/token.sol";

contract CroBridge {

    event __CronosSendToIbc(string recipient, uint256 amount);

    // Pay the contract a certain CRO amount and trigger a CRO transfer
    // from the contract to recipient through IBC
    function send_cro_to_crypto_org(string recipient) public payable {
        emit __CronosSendToIbc(recipient, msg.value);
    }
}
