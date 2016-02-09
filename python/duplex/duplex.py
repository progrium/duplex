
__all__ = [
    'protocol',
    'RPC',
    'Peer',
    'Channel',
    'JSONCodec',
]

import asyncio
import uuid
import json

try:
    spawn = asyncio.ensure_future
except:
    spawn = asyncio.async

class protocol:
    name = "SIMPLEX"
    version = "1.0"
    request = "req"
    reply = "rep"
    class handshake:
        accept = "+OK"

# TODO: put somewhere else?
JSONCodec = ['json', json.dumps, json.loads]

def request_msg(payload, method, id=None, more=None, ext=None):
    msg = dict(
        type=protocol.request,
        method=method,
        payload=payload,
    )
    if id is not None:
        msg['id'] = id
    if more is True:
        msg['more'] = True
    if ext is not None:
        msg['ext'] = ext
    return msg

def reply_msg(id, payload, more=None, ext=None):
    msg = dict(
        type=protocol.reply,
        id=id,
        payload=payload,
    )
    if more is True:
        msg['more'] = True
    if ext is not None:
        msg['ext'] = ext
    return msg

def error_msg(id, code, message, data=None, ext=None):
    msg = dict(
        type=protocol.reply,
        id=id,
        error=dict(
            code=code,
            message=message,
        ),
    )
    if data is not None:
        msg['error']['data'] = data
    if ext is not None:
        msg['ext'] = ext
    return msg

class RPC(object):
    def __init__(self, codec, loop=None):
        self.codec = codec
        self.encode = codec[1]
        self.decode = codec[2]
        self.registered = {}
        if loop is None:
            self.loop = asyncio.get_event_loop()
        else:
            self.loop = loop

    def register(self, method, handler):
        self.registered[method] = handler

    def unregister(self, method):
        del self.registered[method]

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

    def callback_func(self, func):
        name = "_callback.{0}".format(uuid.uuid4())
        self.register_func(name, func)
        return name

    def _handshake(self):
        return "{0}/{1};{2}".format(
            protocol.name,
            protocol.version,
            self.codec[0])

    @asyncio.coroutine
    def handshake(self, conn, route=True):
        peer = Peer(self, conn)
        yield from conn.send(self._handshake())
        resp = yield from conn.recv()
        if resp[0] != "+":
            raise Exception("bad handshake")
        if route:
            peer.route()
        return peer

    @asyncio.coroutine
    def accept(self, conn, route=True):
        peer = Peer(self, conn)
        handshake = yield from conn.recv()
        # TODO: check handshake
        yield from conn.send(protocol.handshake.accept)
        if route:
            peer.route()
        return peer

class Peer(object):
    def __init__(self, rpc, conn):
        self.rpc = rpc
        self.conn = conn
        self.req_chans = {}
        self.rep_chans = {}
        self.counter = 0
        self.tasks = []

    def route(self):
        task = spawn(self._route(), loop=self.rpc.loop)
        self.tasks.append(task)
        return task

    @asyncio.coroutine
    def _route(self, loops=None):
        while True:
            if loops is not None:
                if loops == 0:
                    break
                loops -= 1
            frame = yield from self.conn.recv()
            if frame == "":
                # ignore empty frames
                continue
            try:
                msg = self.rpc.decode(frame)
            except Exception as e:
                # todo: handle decode error
                raise e
            if msg['type'] == protocol.request:
                if msg.get('id') in self.req_chans:
                    if msg.get('more', False) is not True:
                        del self.req_chans[msg['id']]
                else:
                    ch = Channel(self, protocol.reply, msg['method'])
                    if 'id' in msg:
                        ch.id = msg['id']
                        if msg.get('more', False):
                            self.req_chans[ch.id] = ch
                    # TODO: locks
                    if msg['method'] in self.rpc.registered:
                        self.tasks.append(spawn(
                            self.rpc.registered[msg['method']](ch),
                            loop=self.rpc.loop))
                    else:
                        raise Exception("method missing")
                if 'ext' in msg:
                    ch.ext = msg['ext']
                ch.inbox.put_nowait([msg['payload'], msg.get('more', False)])
                if msg.get('more', False) is not True:
                    # ???
                    pass

            elif msg['type'] == protocol.reply:
                if 'error' in msg:
                    ch = self.rep_chans[msg['id']]
                    ch.inbox.put_nowait([msg['error'], False]) # TODO: wrap as exception?
                    del self.rep_chans[msg['id']]
                else:
                    ch = self.rep_chans[msg['id']]
                    ch.inbox.put_nowait([msg['payload'], msg.get('more', False)])
                    if msg.get('more', False) is False:
                        del self.rep_chans[msg['id']]
            else:
                raise Exception("bad msg type: {0}".format(msg.type))

    @asyncio.coroutine
    def close(self):
        for task in self.tasks:
            task.cancel()
        if len(self.tasks) > 0:
            yield from asyncio.wait(self.tasks)
        yield from self.conn.close()

    @asyncio.coroutine
    def call(self, method, args, async=False):
        ch = self.open(method)
        yield from ch.send(args)
        if not async:
            ret, _ = yield from ch.recv()
            # todo: if more is true, error
            return ret
        return None

    def open(self, method):
        ch = Channel(self, protocol.request, method)
        self.counter += 1
        ch.id = self.counter
        self.rep_chans[ch.id] = ch
        return ch


class Channel(object):
    def __init__(self, peer, type_, method):
        self.peer = peer
        self.ext = None
        self.type = type_
        self.method = method
        self.id = None
        self.inbox = asyncio.Queue(loop=peer.rpc.loop)

    @asyncio.coroutine
    def call(self, method, args, async=False):
        ch = self.peer.open(method)
        ch.ext = self.ext
        yield from ch.send(args)
        if not async:
            ret, _ = yield from ch.recv()
            # todo: if more is true, error
            return ret
        return None

    @asyncio.coroutine
    def close(self):
        yield from self.peer.close()

    def open(self, method):
        ch = self.peer.open(method)
        ch.ext = self.ext
        return ch

    @asyncio.coroutine
    def send(self, payload, more=False):
        if self.type == protocol.request:
            yield from self.peer.conn.send(
                self.peer.rpc.encode(request_msg(
                    payload,
                    self.method,
                    self.id,
                    more,
                    self.ext,
                )))
        elif self.type == protocol.reply:
            yield from self.peer.conn.send(
                self.peer.rpc.encode(reply_msg(
                    self.id,
                    payload,
                    more,
                    self.ext,
                )))
        else:
            raise Exception("bad channel type")

    @asyncio.coroutine
    def senderr(self, code, message, data=None):
        if self.type != protocol.reply:
            raise Exception("not a reply channel")
        yield from self.peer.conn.send(
            self.peer.rpc.encode(error_msg(
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
