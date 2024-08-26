"""
client for sync service
"""

import json
import os
import queue
import threading

import websocket

from .params import RunParams


class SyncService:
    def __init__(self, params: RunParams, url=None):
        if not url:
            url = sync_service_url()
        self._id = 0
        self._handlers = {}
        self._quit = False
        self.ws = websocket.create_connection(url)
        self._params = params

        self._recv_thread = threading.Thread(target=self.recv_loop)
        self._recv_thread.start()

    def recv_loop(self):
        while not self._quit:
            rsp = self.ws.recv()
            if not rsp:
                break
            self.on_message(rsp)

        handlers = self._handlers
        self._handlers = None

        for handler in handlers.values():
            handler(None)

    def on_message(self, msg):
        msg = json.loads(msg)
        id = int(msg["id"])
        try:
            callback = self._handlers[id]
        except KeyError:
            return
        callback(msg)

    def close(self):
        self._quit = True
        self.ws.close()
        self._recv_thread.join()

    def next_id(self):
        self._id += 1
        return self._id

    def _request(self, name, payload):
        "send request, wait for response"
        id = self.next_id()
        cond = Condition()
        self._handlers[id] = cond.notify

        self._send(
            {
                "id": str(id),
                name: payload,
            }
        )

        rsp = cond.wait()
        if rsp is None:
            # connection closed
            return
        del self._handlers[id]
        assert not rsp.get("error"), rsp
        return rsp

    def _publish(self, topic, payload=None):
        rsp = self._request(
            "publish",
            {
                "topic": topic,
                "payload": payload,
            },
        )
        return rsp["publish"]["seq"]

    def _subscribe(self, topic, callback):
        "send one request, recv multiple responses"

        def on_recv(msg):
            callback(json.loads(msg["subscribe"]) if msg is not None else None)

        id = self.next_id()
        self._handlers[id] = on_recv
        self._send(
            {
                "id": str(id),
                "subscribe": {
                    "topic": topic,
                },
            }
        )

    def signal_event(self, event):
        return self._publish(self._params.events_key(), event)

    def publish(self, topic, payload=None):
        return self._publish(self._params.topic_key(topic), payload)

    def barrier(self, state, target):
        self._request(
            "barrier",
            {
                "state": self._params.state_key(state),
                "target": target,
            },
        )

    def signal_entry(self, state):
        rsp = self._request(
            "signal_entry",
            {
                "state": self._params.state_key(state),
            },
        )
        return rsp["signal_entry"]["seq"]

    def subscribe(self, topic, callback):
        return self._subscribe(self._params.topic_key(topic), callback)

    def subscribe_simple(self, topic, n):
        """
        subscribe_simple wait for exactly n messages on the topic and return them
        """
        q = queue.Queue()
        self.subscribe(topic, q.put)
        return [q.get() for _ in range(n)]

    def _send(self, payload):
        self.ws.send(json.dumps(payload))

    def publish_and_wait(self, topic, payload, state, target):
        """
        composes Publish and a Barrier. It first publishes the
        provided payload to the specified topic, then awaits for a barrier on the
        supplied state to reach the indicated target.

        If any operation fails, PublishAndWait short-circuits and returns a non-nil
        error and a negative sequence. If Publish succeeds, but the Barrier fails,
        the seq number will be greater than zero.
        """
        seq = self.publish(topic, payload)
        self.barrier(state, target)
        return seq

    def publish_subscribe(self, topic, payload, callback):
        """
        publish_subscribe publishes the payload on the supplied Topic, then subscribes
        to it, sending paylods to the supplied channel.
        """
        seq = self.publish(topic, payload)
        self.subscribe(topic, callback)
        return seq

    def publish_subscribe_simple(self, topic, payload, n):
        self.publish(topic, payload)
        return self.subscribe_simple(topic, n)

    def signal_and_wait(self, state, target):
        seq = self.signal_entry(state)
        self.barrier(state, target)
        return seq


def sync_service_url():
    host = os.environ.get("SYNC_SERVICE_HOST", "testground-sync-service")
    port = os.environ.get("SYNC_SERVICE_PORT", "5050")
    return f"ws://{host}:{port}"


class Condition:
    """
    Condition with payload
    """

    def __init__(self):
        self._cond = threading.Condition()
        self._value = None

    def wait(self):
        with self._cond:
            if self._value is not None:
                return self._value
            self._cond.wait()
            return self._value

    def notify(self, value):
        with self._cond:
            self._value = value
            self._cond.notify()
