import asyncio

from . import protocol
from . import sync

try:
    spawn = asyncio.ensure_future
except:
    spawn = asyncio.async

class Channel(sync.Channel):
    def __init__(self, peer, type_, method, ext=None):
        super().__init__(peer, type_, method, ext)
        self.inbox = asyncio.Queue(loop=peer.rpc.loop)

    @asyncio.coroutine
    def call(self, method, args=None, wait=True):
        ch = self.peer.open(method, self.ext)
        yield from ch.send(args)
        if wait:
            ret, _ = yield from ch.recv()
            # todo: if more is true, error
            return ret
        return None

    @asyncio.coroutine
    def close(self):
        yield from self.peer.close()

    @asyncio.coroutine
    def send(self, payload, more=False):
        if self.type == protocol.types.request:
            yield from self.peer.conn.send(
                self.peer.rpc.encode(protocol.request(
                    payload,
                    self.method,
                    self.id,
                    more,
                    self.ext,
                )))
        elif self.type == protocol.types.reply:
            yield from self.peer.conn.send(
                self.peer.rpc.encode(protocol.reply(
                    self.id,
                    payload,
                    more,
                    self.ext,
                )))
        else:
            raise Exception("bad channel type")

    @asyncio.coroutine
    def senderr(self, code, message, data=None):
        if self.type != protocol.types.reply:
            raise Exception("not a reply channel")
        yield from self.peer.conn.send(
            self.peer.rpc.encode(protocol.error(
                self.id,
                code,
                message,
                data,
                self.ext,
            )))

    @asyncio.coroutine
    def recv(self):
        obj, more = yield from self.inbox.get()
        return [obj, more]


class Peer(sync.Peer):
    Channel = Channel

    def __init__(self, rpc, conn):
        super().__init__(rpc, conn)

    def _spawn(self, fn, *args, **kwargs):
        task = spawn(fn(*args, **kwargs), loop=self.rpc.loop)
        self.tasks.append(task)
        return task

    @asyncio.coroutine
    def route(self, loops=None):
        while self.routing:
            if loops is not None:
                if loops == 0:
                    break
                loops -= 1
            frame = yield from self.conn.recv()
            self._handle_frame(frame)

    @asyncio.coroutine
    def close(self):
        yield from self.conn.close()
        for task in self.tasks:
            task.cancel()
        if len(self.tasks) > 0:
            yield from asyncio.wait(self.tasks)

    @asyncio.coroutine
    def call(self, method, args=None, wait=True):
        ch = self.open(method)
        yield from ch.send(args)
        if wait:
            ret, _ = yield from ch.recv()
            # todo: if more is true, error
            return ret
        return None


class RPC(sync.RPC):
    Peer = Peer

    def __init__(self, codec, loop=None):
        super().__init__(codec)
        if loop is None:
            self.loop = asyncio.get_event_loop()
        else:
            self.loop = loop

    def register_func(self, method, func):
        @asyncio.coroutine
        def func_wrapped(ch):
            args, _ = yield from ch.recv()
            if asyncio.iscoroutinefunction(func):
                ret = yield from func(args, ch)
            else:
                ret = func(args, ch)
            yield from ch.send(ret)
        self.register(method, func_wrapped)

    @asyncio.coroutine
    def handshake(self, conn, route=True):
        peer = Peer(self, conn)
        yield from conn.send(self._handshake())
        resp = yield from conn.recv()
        if resp[0] != "+":
            raise Exception("bad handshake")
        if route:
            peer._spawn(peer.route)
        return peer

    @asyncio.coroutine
    def accept(self, conn, route=True):
        peer = Peer(self, conn)
        handshake = yield from conn.recv()
        # TODO: check handshake
        yield from conn.send(protocol.handshake.accept)
        if route:
            peer._spawn(peer.route)
        return peer
