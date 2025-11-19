import asyncio
import json
import time
from collections import defaultdict

import websockets
from eth_utils import abi
from hexbytes import HexBytes
from pystarport import ports
from web3 import Web3

from .network import Cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    send_raw_transactions,
    send_transaction,
    sign_transaction,
    wait_for_new_blocks,
    wait_for_port,
)


class Client:
    def __init__(self, ws):
        self._ws = ws
        self._gen_id = 0
        self._subs = defaultdict(asyncio.Queue)
        self._rsps = defaultdict(asyncio.Queue)

    def gen_id(self):
        self._gen_id += 1
        return self._gen_id

    async def receive_loop(self):
        while True:
            msg = json.loads(await self._ws.recv())
            if "id" in msg:
                # responses
                await self._rsps[msg["id"]].put(msg)
            else:
                # subscriptions
                assert msg["method"] == "eth_subscription"
                sub_id = msg["params"]["subscription"]
                await self._subs[sub_id].put(msg["params"]["result"])

    async def recv_response(self, rpcid):
        rsp = await self._rsps[rpcid].get()
        del self._rsps[rpcid]
        return rsp

    async def recv_subscription(self, sub_id):
        return await self._subs[sub_id].get()

    async def subscribe(self, *args):
        rpcid = self.gen_id()
        await self._ws.send(
            json.dumps({"id": rpcid, "method": "eth_subscribe", "params": args})
        )
        rsp = await self.recv_response(rpcid)
        assert "error" not in rsp
        return rsp["result"]

    def sub_qsize(self, sub_id):
        return self._subs[sub_id].qsize()

    async def unsubscribe(self, sub_id):
        rpcid = self.gen_id()
        await self._ws.send(
            json.dumps({"id": rpcid, "method": "eth_unsubscribe", "params": [sub_id]})
        )
        rsp = await self.recv_response(rpcid)
        assert "error" not in rsp
        return rsp["result"]


# TestEvent topic from TestMessageCall contract calculated from event signature
TEST_EVENT_TOPIC = Web3.keccak(text="TestEvent(uint256)")


def test_subscribe_basic(cronos: Cronos):
    """
    test basic subscribe and unsubscribe
    """
    wait_for_port(ports.evmrpc_ws_port(cronos.base_port(0)))
    cli = cronos.cosmos_cli()
    loop = asyncio.get_event_loop()

    async def assert_unsubscribe(c: Client, sub_id):
        assert await c.unsubscribe(sub_id)
        # check no more messages
        await loop.run_in_executor(None, wait_for_new_blocks, cli, 2)
        assert c.sub_qsize(sub_id) == 0
        # unsubscribe again return False
        assert not await c.unsubscribe(sub_id)

    async def subscriber_test(c: Client):
        sub_id = await c.subscribe("newHeads")
        # wait for three new blocks
        msgs = [await c.recv_subscription(sub_id) for i in range(3)]
        # check blocks are consecutive
        assert int(msgs[1]["number"], 0) == int(msgs[0]["number"], 0) + 1
        assert int(msgs[2]["number"], 0) == int(msgs[1]["number"], 0) + 1
        await assert_unsubscribe(c, sub_id)

    async def transfer_test(c: Client, w3, contract, address):
        sub_id = await c.subscribe("logs", {"address": address})
        to = ADDRS["community"]
        _from = ADDRS["validator"]
        total = 5
        topic = abi.event_signature_to_log_topic("Transfer(address,address,uint256)")
        for i in range(total):
            amt = 10 + i
            tx = contract.functions.transfer(to, amt).build_transaction({"from": _from})
            txreceipt = send_transaction(w3, tx)
            assert len(txreceipt.logs) == 1
            expect_log = {
                "address": address,
                "topics": [
                    HexBytes(topic),
                    HexBytes(b"\x00" * 12 + HexBytes(_from)),
                    HexBytes(b"\x00" * 12 + HexBytes(to)),
                ],
                "data": HexBytes(b"\x00" * 31 + HexBytes(amt)),
            }
            assert expect_log.items() <= txreceipt.logs[0].items()
        msgs = [await c.recv_subscription(sub_id) for i in range(total)]
        assert len(msgs) == total
        await assert_unsubscribe(c, sub_id)

    async def logs_test(c: Client, w3, contract, address):
        sub_id = await c.subscribe("logs", {"address": address})
        iterations = 10000
        tx = contract.functions.test(iterations).build_transaction()
        raw_transactions = []
        for key_from in KEYS.values():
            signed = sign_transaction(w3, tx, key_from)
            raw_transactions.append(signed.raw_transaction)
        send_raw_transactions(w3, raw_transactions)
        total = len(KEYS) * iterations
        msgs = [await c.recv_subscription(sub_id) for i in range(total)]
        assert len(msgs) == total
        assert all(msg["topics"] == [Web3.to_hex(TEST_EVENT_TOPIC)] for msg in msgs)
        await assert_unsubscribe(c, sub_id)

    async def async_test():
        async with websockets.connect(cronos.w3_ws_endpoint()) as ws:
            c = Client(ws)
            t = asyncio.create_task(c.receive_loop())
            # run three subscribers concurrently
            await asyncio.gather(*[subscriber_test(c) for i in range(3)])
            contract = deploy_contract(cronos.w3, CONTRACTS["TestERC20A"])
            address = contract.address
            await transfer_test(c, cronos.w3, contract, address)
            contract = deploy_contract(cronos.w3, CONTRACTS["TestMessageCall"])
            inner = contract.caller.inner()
            begin = time.time()
            await logs_test(c, cronos.w3, contract, inner)
            print("msg call time", time.time() - begin)
            t.cancel()
            try:
                await t
            except asyncio.CancelledError:
                pass

    timeout = 100
    loop.run_until_complete(asyncio.wait_for(async_test(), timeout))
