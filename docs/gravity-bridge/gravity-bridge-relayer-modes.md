## Gravity Bridge Relayer Modes

Gravity Bridge relayer supports the following modes:
1. AlwaysRelay
2. Api
3. File


### Always relay mode

Using this mode, the relayer will always relay the batches ignoring the cost, profitable or not.

### Api mode

The env variable `RELAYER_API_URL` needs to be set to use this mode. The relayer will call an API that will estimate the cost of sending the batch, check if the daily limit is reached and none of the addresses are blacklisted. It will send the batch only if all the conditions are met.
### File mode

A file `token_prices_json` needs to be present in running directory to use this mode. The relayer will fetch the price of each tokens supported using the file as a data source and relay the batch only if it is profitable.