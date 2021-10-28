pragma solidity >0.5.0;

contract Greeter {
    string public greeting;

    constructor() public {
        greeting = "Hello";
    }

    function setGreeting(string memory _greeting) public {
        greeting = _greeting;
    }

    function greet() public view returns (string memory) {
        return greeting;
    }
}
