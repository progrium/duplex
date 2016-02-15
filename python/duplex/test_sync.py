import unittest
import json
import threading
import queue
import concurrent.futures as futures

from . import RPC
from . import protocol

def spawn(fn, *args, **kwargs):
    f = futures.Future()
    def target(*args, **kwargs):
        f.set_result(fn(*args, **kwargs))
    t = threading.Thread(target=target, args=args, kwargs=kwargs)
    t.start()
    return f

class WaitGroup(object):
    def __init__(self):
        self.total = []
        self.pending = []

    def add(self, incr=1):
        for n in range(incr):
            f = futures.Future()
            self.total.append(f)
            self.pending.append(f)

    def done(self):
        if len(self.pending) == 0:
            return
        f = self.pending.pop()
        f.set_result(True)

    def wait(self):
        futures.wait(self.total)


class MockConnection(object):
    def __init__(self):
        self.sent = []
        self.closed = False
        self.pairedWith = None
        self.expectedSends = WaitGroup()
        self.expectedRecvs = WaitGroup()
        self.inbox = queue.Queue()

    def close(self):
        self.closed = True

    def send(self, frame):
        self.sent.append(frame)
        if self.pairedWith:
            self.pairedWith.inbox.put(frame)
        self.expectedSends.done()

    def recv(self):
        self.expectedRecvs.done()
        return self.inbox.get(timeout=1)

def connection_pair():
    conn1 = MockConnection()
    conn2 = MockConnection()
    conn1.pairedWith = conn2
    conn2.pairedWith = conn1
    return [conn1, conn2]

def peer_pair(rpc):
    conn1, conn2 = connection_pair()
    tasks = [
        spawn(rpc.accept, conn1, False),
        spawn(rpc.handshake, conn2, False),
    ]
    yield from futures.wait(tasks)
    return [tasks[0].result(), tasks[1].result()]

def handshake(codec):
    return "{0}/{1};{2}".format(
        protocol.name,
        protocol.version,
        codec)

def echo(ch):
    obj, _ = ch.recv()
    ch.send(obj)


class DuplexSyncTests(unittest.TestCase):

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def spawn(self, *args, **kwargs):
        return spawn(*args, **kwargs)

    def test_handshake(self):
        conn = MockConnection()
        rpc = RPC("json", async=False)
        conn.inbox.put(protocol.handshake.accept)
        rpc.handshake(conn, False)
        conn.close()
        self.assertEqual(conn.sent[0], handshake("json"))

    def test_accept(self):
        conn = MockConnection()
        rpc = RPC("json", async=False)
        conn.inbox.put(handshake("json"))
        rpc.accept(conn, False)
        conn.close()
        self.assertEqual(conn.sent[0], protocol.handshake.accept)

    def test_all_on_paired_peers(self):
        conns = connection_pair()
        rpc = RPC("json", async=False)
        def echo_tag(ch):
            obj, _ = ch.recv()
            obj["tag"] = True
            ch.send(obj)
        rpc.register("echo-tag", echo_tag)
        tasks = [
            self.spawn(rpc.accept, conns[0], False),
            self.spawn(rpc.handshake, conns[1], False),
        ]
        futures.wait(tasks)
        peer1 = tasks[0].result()
        peer2 = tasks[1].result()
        tasks = [
            self.spawn(peer1.call, "echo-tag", {"from": "peer1"}),
            self.spawn(peer2.call, "echo-tag", {"from": "peer2"}),
            self.spawn(peer1.route, 2),
            self.spawn(peer2.route, 2),
        ]
        futures.wait(tasks)
        conns[0].close()
        conns[1].close()
        self.assertEqual(tasks[0].result()["from"], "peer1")
        self.assertEqual(tasks[0].result()["tag"], True)
        self.assertEqual(tasks[1].result()["from"], "peer2")
        self.assertEqual(tasks[1].result()["tag"], True)
