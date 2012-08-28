"""Duplex: Socket joining

This is a functional prototype in Python for phase 1 of Duplex. In true form, it
would be written in C and have various performance tricks. The C library
would then be wrapped by various language bindings, making this high
performance socket joining library accessible in many languages.
"""
import threading
import select
import socket
import os

# Flags used in public API
HALFDUPLEX = 1
NOCLOSE = 2

class JoinStream(object):
    """One-way stream from one socket to another"""

    socket_from = None
    socket_to = None
    transform = None
    link_close = True

    def __init__(self, ctx, from_, to, transform=None, link_close=True):
        self.socket_from = ctx.managed_socket(from_)
        self.socket_from.streams_out.append(self)

        self.socket_to = ctx.managed_socket(to)
        self.socket_to.streams_in.append(self)

        self.transform = transform
        self.link_close = link_close

    def stop(self, close=False):
        self.socket_from.streams_out.remove(self)
        if close and self.link_close:
            self.socket_from.mark_to_close()
        self.socket_from = None

        self.socket_to.streams_in.remove(self)
        if close and self.link_close:
            self.socket_to.mark_to_close()
        self.socket_to = None

    def __repr__(self):
        return "<{}|{}>".format(self.socket_from, self.socket_to)



class ManagedSocket(object):
    socket = None
    streams_out = None
    streams_in = None
    write_buffer = None
    close_ready = False

    def __init__(self, socket):
        self.streams_out = []
        self.streams_in = []
        self.write_buffer = bytearray()
        self.socket = socket
        self.socket.setblocking(0)
        #print "new managed socket: {}".format(socket)

    def __call__(self):
        return self.socket

    @property
    def is_listening(self):
        try:
            listening = self.socket.getsockopt(socket.SOL_SOCKET, socket.SO_ACCEPTCONN)
        except socket.error:
            listening = -1
        return listening > 0

    def accept(self, ctx):
        """new connections inherit joins on listening socket"""
        conn, address = self.socket.accept()
        for out_stream in self.streams_out:
            JoinStream(ctx, conn, out_stream.socket_to(),
                        transform=out_stream.transform,
                        link_close=out_stream.link_close)
        for in_stream in self.streams_in:
            JoinStream(ctx, in_stream.socket_from(), conn,
                        transform=in_stream.transform,
                        link_close=in_stream.link_close)

    def pump(self, writables):
        # This is the part that would be optimized by
        # splice or sendfile
        try:
            data = self.socket.recv(4096)
            if data:
                for stream in self.streams_out:
                    out = stream.socket_to
                    if out:
                        if stream.transform is not None:
                            d = stream.transform(data)
                        else:
                            d = data
                        if out() in writables and not out.write_buffer:
                            out().send(d)
                        else:
                            out.write_buffer.extend(d)
            else:
                self.mark_to_close()
        except socket.error:
            # lets just assume it died
            self.mark_to_close()

    def flush_write_buffer(self):
        assert self.write_buffer
        bytes = self.socket.send(self.write_buffer)
        self.write_buffer = self.write_buffer[bytes:]

    def mark_to_close(self):
        assert self.socket
        #print "socket marked to close: {}".format(self.socket)
        self.close_ready = True

    def close(self):
        assert self.close_ready
        assert not self.write_buffer
        #print "closing socket: {}".format(self.socket)
        for stream in self.streams_out:
            stream.stop(close=True)
        for stream in self.streams_in:
            stream.stop(close=True)
        self.socket.close()
        socket = self.socket
        self.socket = None
        return socket


class Context(object):
    instance = None

    sockets = {}
    thread = None
    active = True
    join_queue = None
    unjoin_queue = None

    def __init__(self):
        self.sockets = {}
        self.join_queue = []
        self.unjoin_queue = []
        self.thread = threading.Thread(target=self.loop)
        self.thread.start()

    def managed_socket(self, socket):
        if socket not in self.sockets:
            self.sockets[socket] = ManagedSocket(socket)
        return self.sockets[socket]

    def join(self, a, b, transform=None, link_close=True, half_duplex=False):
        if half_duplex:
            JoinStream(self, a, b, transform, link_close)
        else:
            JoinStream(self, a, b, transform, link_close)
            JoinStream(self, b, a, transform, link_close)

    def unjoin(self, a, b):
        if a in self.sockets and b in self.sockets:
            for s in self.sockets[a].streams_out:
                if self.sockets[b] == s.socket_to:
                    s.stop()
            for s in self.sockets[b].streams_in:
                if self.sockets[b] == s.socket_from:
                    s.stop()

    def loop(self):
        while self.active:
            self.do_api_ops()

            readables, writables, errs = select.select(
                                            self.readables(),
                                            self.writables(),
                                            [], 0)
            
            self.flush_buffered_writables(writables)

            for readable in readables:
                managed = self.managed_socket(readable)
                if managed.is_listening:
                    managed.accept(self)
                else:
                    managed.pump(writables) 

            self.close_finished_sockets()

    def do_api_ops(self):
        join_ops = self.join_queue[:]
        self.join_queue = []
        for args in join_ops:
            self.join(*args)

        unjoin_ops = self.unjoin_queue[:]
        self.unjoin_queue = []
        for args in unjoin_ops:
            self.unjoin(*args)

    def readables(self):
        return [socket() for socket in self.sockets.values() if
                len(socket.streams_out)]

    def writables(self):
        return [socket() for socket in self.sockets.values() if
                len(socket.streams_in)]

    def close_finished_sockets(self):
        for socket in self.sockets.values():
            if socket.close_ready and not socket.write_buffer:
                self.sockets.pop(socket.close())

    def flush_buffered_writables(self, writables):
        for socket in self.sockets.values():
            if socket.write_buffer and socket() in writables:
                while len(socket.write_buffer):
                    try:
                        #print "flushing"
                        socket.flush_write_buffer()
                    except Exception:
                        break
                

def shutdown():
    if Context.instance:
        Context.instance.active = False
        Context.instance.thread.join()

def join(a, b, flags=0, transform=None):
    link_close = not flags & NOCLOSE
    half_duplex = flags & HALFDUPLEX

    if Context.instance is None:
        Context.instance = Context()

    Context.instance.join_queue.append(
            (a, b, transform, link_close, half_duplex))

def unjoin(a, b):
    if Context.instance:
        Context.instance.unjoin_queue.append((a, b))

