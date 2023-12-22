// SPDX-License-Identifier: MIT

pragma solidity 0.8.21;

/**
 * @dev Interface of the CosmosToken deployed by the gravity contract
 */
interface ICosmosToken {
    /**
    * @dev Sets the gravity contract and transfer the balance to the new address.
	 *
	 * Requirements:
	 *
	 * - `msg.sender` must be the current gravity contract
	 */
    function setGravityContract(address _gravityAddress) external;

}