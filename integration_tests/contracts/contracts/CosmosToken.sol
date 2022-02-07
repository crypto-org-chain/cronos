pragma solidity ^0.8.10;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract CosmosERC20 is ERC20 {
	uint256 private MAX_UINT = 2**256 - 1;

	address public gravity;

	uint8 private cosmosDecimals;

	mapping(address => mapping(address => uint256)) private _allowances;


	modifier onlyGravity() {
		require(msg.sender == gravity, "Not gravity");
		_;
	}

	constructor(
		address _gravityAddress,
		string memory _name,
		string memory _symbol,
		uint8 _decimals
	) public ERC20(_name, _symbol)  {
		cosmosDecimals = _decimals;
		gravity = _gravityAddress;
		_mint(_gravityAddress, MAX_UINT);

	}


	// This is not an accurate total supply. Instead this is the total supply
	// of the given cosmos asset on Ethereum at this moment in time. Keeping
	// a totally accurate supply would require constant updates from the Cosmos
	// side, while in theory this could be piggy-backed on some existing bridge
	// operation it's a lot of complextiy to add so we chose to forgoe it.
	
	/**
	 * @dev Returns the number of tokens not currently held by the gravity address
	 *

	 */	
	function totalSupply() public view virtual override returns (uint256) {
		return MAX_UINT - balanceOf(gravity);
	}


	/**
	 * @dev Sets the gravity contract to a new address.
	 *
	 * Requirements:
	 *
	 * - `msg.sender` must be the current gravity contract
	 */
	function setGravityContract(address _gravityAddress) external onlyGravity {

		gravity = _gravityAddress;
	 }

	/**
	 * @dev Overrides the decimal function in the base ERC20 contract. 
	 * This override is needed to Ethereum wallets display tokens consistently
	 * with how Cosmos wallets display the native version of the token.
	 */

   function decimals()public view override returns (uint8){
	   return cosmosDecimals;
   }

}
