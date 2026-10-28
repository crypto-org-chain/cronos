import { defineConfig } from "hardhat/config";

export default defineConfig({
  solidity: {
    compilers: [
      {
        version: "0.8.28",
        settings: {
          optimizer: {
            enabled: true
          },
          evmVersion: "prague"
        }
      },
      {
        version: "0.8.21",
        settings: {
          optimizer: {
            enabled: true
          },
          evmVersion: "shanghai"
        }
      },
      {
        version: "0.8.10",
        settings: {
          optimizer: {
            enabled: true
          }
        }
      },
      {
        version: "0.6.6",
        settings: {
          optimizer: {
            enabled: true
          }
        }
      },
    ],
  },
});
