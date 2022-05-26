import json
import sys

result = json.load(sys.stdin)["result"]
for tx in result["txs_results"] or []:
    if (
        tx["code"] == 11
        and "out of gas in location: block gas meter; gasWanted:" in tx["log"]
        and not any(evt["type"] == "ethereum_tx" for evt in tx["events"])
    ):
        print(result["height"])
        break
