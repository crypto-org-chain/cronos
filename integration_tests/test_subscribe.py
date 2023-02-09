import asyncio
import json
from collections import defaultdict

import websockets

from .network import Cronos
from .utils import wait_for_new_blocks


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


def test_subscribe_basic(cronos: Cronos):
    """
    test basic subscribe and unsubscribe
    """
    cli = cronos.cosmos_cli()
    loop = asyncio.get_event_loop()

    async def subscriber_test(c: Client):
        sub_id = await c.subscribe("newHeads")
        # wait for three new blocks
        msgs = [await c.recv_subscription(sub_id) for i in range(3)]
        # check blocks are consecutive
        assert int(msgs[1]["number"], 0) == int(msgs[0]["number"], 0) + 1
        assert int(msgs[2]["number"], 0) == int(msgs[1]["number"], 0) + 1
        assert await c.unsubscribe(sub_id)
        # check no more messages
        await loop.run_in_executor(None, wait_for_new_blocks, cli, 2)
        assert c.sub_qsize(sub_id) == 0
        # unsubscribe again return False
        assert not await c.unsubscribe(sub_id)

    async def async_test():
        async with websockets.connect(cronos.w3_ws_endpoint()) as ws:
            c = Client(ws)
            t = asyncio.create_task(c.receive_loop())
            # run three subscribers concurrently
            await asyncio.gather(*[subscriber_test(c) for i in range(3)])
            t.cancel()
            try:
                await t
            except asyncio.CancelledError:
                pass

    loop.run_until_complete(async_test())
