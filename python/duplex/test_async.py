import asyncio
import unittest
import json
import base64

from . import RPC
from . import protocol

try:
    spawn = asyncio.ensure_future
except:
    spawn = asyncio.async

class WaitGroup(object):
    def __init__(self, loop):
        self.loop = loop
        self.total = []
        self.pending = []

    def add(self, incr=1):
        for n in range(incr):
            f = asyncio.Future(loop=self.loop)
            self.total.append(f)
            self.pending.append(f)

    def done(self):
        if len(self.pending) == 0:
            return
        f = self.pending.pop()
        f.set_result(True)

    @asyncio.coroutine
    def wait(self):
        yield from asyncio.wait(self.total)


class MockConnection(object):
    def __init__(self, loop):
        self.sent = []
        self.closed = False
        self.pairedWith = None
        self.expectedSends = WaitGroup(loop)
        self.expectedRecvs = WaitGroup(loop)
        self.inbox = asyncio.Queue(loop=loop)

    @asyncio.coroutine
    def close(self):
        self.closed = True

    @asyncio.coroutine
    def send(self, frame):
        self.sent.append(frame)
        if self.pairedWith:
            yield from self.pairedWith.inbox.put(frame)
        self.expectedSends.done()


    @asyncio.coroutine
    def recv(self):
        self.expectedRecvs.done()
        frame = yield from self.inbox.get()
        return frame

def connection_pair(loop):
    conn1 = MockConnection(loop)
    conn2 = MockConnection(loop)
    conn1.pairedWith = conn2
    conn2.pairedWith = conn1
    return [conn1, conn2]

@asyncio.coroutine
def peer_pair(loop, rpc):
    conn1, conn2 = connection_pair(loop)
    tasks = [
        spawn(rpc.accept(conn1, False), loop=loop),
        spawn(rpc.handshake(conn2, False), loop=loop),
    ]
    yield from asyncio.wait(tasks)
    return [tasks[0].result(), tasks[1].result()]

def handshake(codec):
    return "{0}/{1};{2}".format(
        protocol.name,
        protocol.version,
        codec)

@asyncio.coroutine
def echo(ch):
    obj, _ = yield from ch.recv()
    yield from ch.send(obj)


class DuplexAsyncTests(unittest.TestCase):

    def setUp(self):
        self.loop = asyncio.new_event_loop()
        asyncio.set_event_loop(self.loop)
        self.close = [self.loop]

    def tearDown(self):
        for obj in self.close:
            obj.close()

    def spawn(self, *args, **kwargs):
        kwargs['loop'] = self.loop
        return spawn(*args, **kwargs)

    def async(f):
        def wrapper(*args, **kwargs):
            coro = asyncio.coroutine(f)
            future = coro(*args, **kwargs)
            args[0].loop.run_until_complete(
                asyncio.wait_for(future, 5, loop=args[0].loop))
        return wrapper

    @async
    def test_handshake(self):
        conn = MockConnection(self.loop)
        rpc = RPC("json", self.loop)
        yield from conn.inbox.put(protocol.handshake.accept)
        yield from rpc.handshake(conn, False)
        yield from conn.close()
        self.assertEqual(conn.sent[0], handshake("json"))

    @async
    def test_accept(self):
        conn = MockConnection(self.loop)
        rpc = RPC("json", self.loop)
        yield from conn.inbox.put(handshake("json"))
        yield from rpc.accept(conn, False)
        yield from conn.close()
        self.assertEqual(conn.sent[0], protocol.handshake.accept)

    @async
    def test_registered_func_after_accept(self):
        conn = MockConnection(self.loop)
        conn.expectedSends.add(2)
        rpc = RPC("json", self.loop)
        rpc.register("echo", echo)
        yield from conn.inbox.put(handshake("json"))
        peer = yield from rpc.accept(conn, False)
        req = {
            'type': protocol.types.request,
            'method': "echo",
            'id': 1,
            'payload': {"foo": "bar"}
        }
        frame = json.dumps(req)
        yield from conn.inbox.put(frame)
        yield from peer.route(1)
        yield from conn.expectedSends.wait()
        yield from conn.close()
        self.assertEqual(len(conn.sent), 2)

    @async
    def test_registered_func_after_handshake(self):
        conn = MockConnection(self.loop)
        conn.expectedSends.add(2)
        rpc = RPC("json", self.loop)
        rpc.register("echo", echo)
        yield from conn.inbox.put(protocol.handshake.accept)
        peer = yield from rpc.handshake(conn, False)
        req = {
            'type': protocol.types.request,
            'method': "echo",
            'id': 1,
            'payload': {"foo": "bar"}
        }
        frame = json.dumps(req)
        yield from conn.inbox.put(frame)
        yield from peer.route(1)
        yield from conn.expectedSends.wait()
        yield from conn.close()
        self.assertEqual(len(conn.sent), 2)

    @async
    def test_call_after_handshake(self):
        conn = MockConnection(self.loop)
        conn.expectedSends.add(2)
        rpc = RPC("json", self.loop)
        yield from conn.inbox.put(protocol.handshake.accept)
        peer = yield from rpc.handshake(conn, False)
        args = {"foo": "bar"}
        expectedReply = {"baz": "qux"}
        frame = json.dumps({
            "type": protocol.types.reply,
            "id": 1,
            "payload": expectedReply,
        })
        @asyncio.coroutine
        def inject_frame():
            yield from conn.expectedSends.wait()
            yield from conn.inbox.put(frame)
        tasks = [
            self.spawn(peer.call("foobar", args)),
            self.spawn(inject_frame()),
            peer.route(1),
        ]
        yield from asyncio.wait(tasks)
        reply = tasks[0].result()
        yield from conn.close()
        self.assertEqual(reply["baz"], expectedReply["baz"])

    @async
    def test_call_after_accept(self):
        conn = MockConnection(self.loop)
        conn.expectedSends.add(2)
        rpc = RPC("json", self.loop)
        yield from conn.inbox.put(handshake("json"))
        peer = yield from rpc.accept(conn, False)
        args = {"foo": "bar"}
        expectedReply = {"baz": "qux"}
        frame = json.dumps({
            "type": protocol.types.reply,
            "id": 1,
            "payload": expectedReply,
        })
        @asyncio.coroutine
        def inject_frame():
            yield from conn.expectedSends.wait()
            yield from conn.inbox.put(frame)
        tasks = [
            self.spawn(peer.call("foobar", args)),
            self.spawn(inject_frame()),
            peer.route(1),
        ]
        yield from asyncio.wait(tasks)
        reply = tasks[0].result()
        yield from conn.close()
        self.assertEqual(reply["baz"], expectedReply["baz"])

    @async
    def test_all_on_paired_peers(self):
        conns = connection_pair(self.loop)
        rpc = RPC("json", self.loop)
        @asyncio.coroutine
        def echo_tag(ch):
            obj, _ = yield from ch.recv()
            obj["tag"] = True
            yield from ch.send(obj)
        rpc.register("echo-tag", echo_tag)
        tasks = [
            self.spawn(rpc.accept(conns[0], False)),
            self.spawn(rpc.handshake(conns[1], False)),
        ]
        yield from asyncio.wait(tasks)
        peer1 = tasks[0].result()
        peer2 = tasks[1].result()
        tasks = [
            self.spawn(peer1.call("echo-tag", {"from": "peer1"})),
            self.spawn(peer2.call("echo-tag", {"from": "peer2"})),
            peer1.route(2),
            peer2.route(2),
        ]
        yield from asyncio.wait(tasks + peer1.tasks + peer2.tasks)
        yield from conns[0].close()
        yield from conns[1].close()
        self.assertEqual(tasks[0].result()["from"], "peer1")
        self.assertEqual(tasks[0].result()["tag"], True)
        self.assertEqual(tasks[1].result()["from"], "peer2")
        self.assertEqual(tasks[1].result()["tag"], True)

    @async
    def test_streaming_multiple_results(self):
        rpc = RPC("json", self.loop)
        @asyncio.coroutine
        def counter(ch):
            count, _ = yield from ch.recv()
            for i in range(count):
                n = i+1
                yield from ch.send({"num": n}, n != count)
        rpc.register("count", counter)
        client, server = yield from peer_pair(self.loop, rpc)
        ch = client.open("count")
        yield from ch.send(5)
        yield from server.route(1)
        @asyncio.coroutine
        def handle_results():
            more = True
            loops = 0
            count = 0
            while more:
                reply, more = yield from ch.recv()
                count += reply['num']
                loops += 1
                assert loops <= 5
            return count
        tasks = [
            self.spawn(handle_results()),
            client.route(5), # kinda defeats the point
        ]
        yield from asyncio.wait(tasks + server.tasks)
        yield from client.close()
        yield from server.close()
        self.assertEqual(tasks[0].result(), 15)

    @async
    def test_streaming_multiple_arguments(self):
        rpc = RPC("json", self.loop)
        @asyncio.coroutine
        def adder(ch):
            more = True
            total = 0
            while more:
                count, more = yield from ch.recv()
                total += count
            yield from ch.send(total)
        rpc.register("adder", adder)
        client, server = yield from peer_pair(self.loop, rpc)
        ch = client.open("adder")
        @asyncio.coroutine
        def asyncio_sucks():
            for i in range(5):
                n = i+1
                yield from ch.send(n, n != 5)
            total, _ = yield from ch.recv()
            return total
        tasks = [
            self.spawn(asyncio_sucks()),
            server.route(5),
            client.route(1),
        ]
        yield from asyncio.wait(tasks + server.tasks)
        yield from client.close()
        yield from server.close()
        self.assertEqual(tasks[0].result(), 15)

    @async
    def test_custom_codec(self):
        b64json = [
            'b64json',
            lambda obj: base64.b64encode(json.dumps(obj).encode("utf-8")).decode("utf-8"),
            lambda s: json.loads(base64.b64decode(s.encode("utf-8")).decode("utf-8")),
        ]
        rpc = RPC(b64json, self.loop)
        rpc.register("echo", echo)
        client, server = yield from peer_pair(self.loop, rpc)
        args = {"foo": "bar"}
        tasks = [
            self.spawn(client.call("echo", args)),
            server.route(1),
            client.route(1),
        ]
        yield from asyncio.wait(tasks + server.tasks)
        yield from client.close()
        yield from server.close()
        self.assertEqual(tasks[0].result(), args)

    @async
    def test_ext_fields(self):
        rpc = RPC("json", self.loop)
        rpc.register("echo", echo)
        client, server = yield from peer_pair(self.loop, rpc)
        args = {"foo": "bar"}
        ext = {"hidden": "metadata"}
        ch = client.open("echo")
        ch.ext = ext
        yield from ch.send(args)
        yield from server.route(1)
        yield from client.route(1)
        reply, _ = yield from ch.recv()
        self.assertEqual(args, reply)
        yield from client.close()
        yield from server.close()
        msg = json.loads(server.conn.sent[1])
        self.assertEqual(msg['ext'], ext)

    @async
    def test_register_func_and_callback_func(self):
        rpc = RPC("json", self.loop)
        @asyncio.coroutine
        def callback(args, ch):
            ret = yield from ch.call(args[0], args[1])
            return ret
        rpc.register_func("callback", callback)
        client, server = yield from peer_pair(self.loop, rpc)
        upper = rpc.callback_func(lambda arg,ch: arg.upper())
        tasks = [
            self.spawn(client.call("callback", [upper, "hello"])),
            server.route(2),
            client.route(2),
        ]
        yield from asyncio.wait(tasks)
        yield from client.close()
        yield from server.close()
        self.assertEqual(tasks[0].result(), "HELLO")
