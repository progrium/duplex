import uuid
import queue
import threading

from . import protocol
from . import codecs

class Channel(object):
    def __init__(self, peer, type_, method, ext=None):
        self.peer = peer
        self.ext = ext
        self.type = type_
        self.method = method
        self.id = None
        self.inbox = queue.Queue()

    def call(self, method, args=None, wait=True):
        ch = self.peer.open(method, self.ext)
        ch.send(args)
        if wait:
            ret, _ = ch.recv()
            # todo: if more is true, error
            return ret
        return None

    def close(self):
        self.peer.close()

    def open(self, method):
        ch = self.peer.open(method, self.ext)
        return ch

    def send(self, payload, more=False):
        if self.type == protocol.types.request:
            self.peer.conn.send(
                self.peer.rpc.encode(protocol.request(
                    payload,
                    self.method,
                    self.id,
                    more,
                    self.ext,
                )))
        elif self.type == protocol.types.reply:
            self.peer.conn.send(
                self.peer.rpc.encode(protocol.reply(
                    self.id,
                    payload,
                    more,
                    self.ext,
                )))
        else:
            raise Exception("bad channel type")

    def senderr(self, code, message, data=None):
        if self.type != protocol.types.reply:
            raise Exception("not a reply channel")
        self.peer.conn.send(
            self.peer.rpc.encode(protocol.error(
                self.id,
                code,
                message,
                data,
                self.ext,
            )))

    def recv(self, timeout=None):
        return self.inbox.get(timeout=timeout)


class Peer(object):
    Channel = Channel

    def __init__(self, rpc, conn):
        self.rpc = rpc
        self.conn = conn
        self.req_chans = {}
        self.rep_chans = {}
        self.counter = 0
        self.tasks = []
        self.routing = True

    def _spawn(self, fn, *args, **kwargs):
        t = threading.Thread(target=fn, args=args, kwargs=kwargs)
        self.tasks.append(t)
        t.start()
        return t

    def route(self, loops=None):
        while self.routing:
            if loops is not None:
                if loops == 0:
                    break
                loops -= 1
            frame = self.conn.recv()
            self._handle_frame(frame)

    def _handle_frame(self, frame):
        if frame == "":
            # ignore empty frames
            return
        try:
            msg = self.rpc.decode(frame)
        except Exception as e:
            # todo: handle decode error
            raise e
        if msg['type'] == protocol.types.request:
            if msg.get('id') in self.req_chans:
                ch = self.req_chans[msg['id']]
                if msg.get('more', False) is not True:
                    del self.req_chans[msg['id']]
            else:
                ch = self.Channel(self, protocol.types.reply, msg['method'])
                if 'id' in msg:
                    ch.id = msg['id']
                    if msg.get('more', False):
                        self.req_chans[ch.id] = ch
                # TODO: locks
                if msg['method'] in self.rpc.registered:
                    self._spawn(self.rpc.registered[msg['method']], ch)
                else:
                    raise Exception("method missing")
            if 'ext' in msg:
                ch.ext = msg['ext']
            ch.inbox.put_nowait([msg['payload'], msg.get('more', False)])
            if msg.get('more', False) is not True:
                # ???
                pass

        elif msg['type'] == protocol.types.reply:
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


    def close(self):
        self.routing = False
        self.conn.close()
        for task in self.tasks:
            task.join()

    def call(self, method, args=None, wait=True):
        ch = self.open(method)
        ch.send(args)
        if wait:
            ret, _ = ch.recv()
            # todo: if more is true, error
            return ret
        return None

    def open(self, method, ext=None):
        ch = self.Channel(self, protocol.types.request, method, ext)
        self.counter += 1
        ch.id = self.counter
        self.rep_chans[ch.id] = ch
        return ch


class RPC(object):
    Peer = Peer

    def __init__(self, codec):
        if isinstance(codec, str):
            self.codec = codecs.load(codec)
        else:
            self.codec = codec
        self.encode = self.codec[1]
        self.decode = self.codec[2]
        self.registered = {}

    def register(self, method, handler):
        self.registered[method] = handler

    def unregister(self, method):
        del self.registered[method]

    def register_func(self, method, func):
        def func_wrapped(ch):
            args, _ = ch.recv()
            ret = func(args, ch)
            ch.send(ret)
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

    def handshake(self, conn, route=True):
        peer = self.Peer(self, conn)
        conn.send(self._handshake())
        resp = conn.recv()
        if resp[0] != "+":
            raise Exception("bad handshake")
        if route:
            peer._spawn(peer.route)
        return peer

    def accept(self, conn, route=True):
        peer = self.Peer(self, conn)
        handshake = conn.recv()
        # TODO: check handshake
        conn.send(protocol.handshake.accept)
        if route:
            peer._spawn(peer.route)
        return peer
