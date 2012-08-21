"""Megasock prototype

This is a functional prototype in Python for Megasock. In true form, it
would be written in C and have various performance tricks. The C library
would then be wrapped by various language bindings, making this high
performance socket joining library accessible in many languages.
"""
import fcntl
import threading
import select
import socket
import os

# Flags
HALFDUPLEX = 1


class _context:
    """data for a context thread

    the only non-obvious thing is the joins structure. it is a dict of
    join information. the keys are file descriptors/numbers for
    anything that has been joined. the value is a two value tuple where
    the first element is a list of socket objects that are read into
    that file descriptor, and the second element is a list of socket
    objects that are written to from that file descriptor.

    this is used to provide two methods, readables and writables, that
    alone provide all socket objects for reading and writing for use
    with select. they can also take an argument to find all the
    readables or writables for a particular file descriptor, which is
    used for determining where to recv and where to send.
    """
    thread = None
    joins = {}
    callbacks = {}
    active = True

    def _join_lookup(self, rw_idx, fd=None):
        if fd is None:
            return [item for j in self.joins.values() 
                                            for item in j[rw_idx]]
        else:
            return self.joins[fd.fileno()][rw_idx]

    def readables(self, fd=None): return self._join_lookup(0, fd)
    def writables(self, fd=None): return self._join_lookup(1, fd)


def _context_loop(ctx):
    while ctx.active:
        readables, writables, errs = select.select(
                                        ctx.readables(),
                                        ctx.writables(),
                                        [], 0)

        for readable in readables:
            try:
                listening = readable.getsockopt(socket.SOL_SOCKET, socket.SO_ACCEPTCONN)
            except socket.error:
                listening = -1
            if listening > 0:
                conn, address = readable.accept()
                conn.setblocking(0)
                # joins on accepting sockets are inherited by
                # their accepted connection sockets
                ctx.joins[conn.fileno()] = (
                        list(ctx.joins[readable.fileno()][0]),
                        list(ctx.joins[readable.fileno()][1]))
                for r in ctx.joins[readable.fileno()][0]:
                    ctx.joins[r.fileno()][1].append(conn)
                    if (r.fileno(), readable.fileno()) in ctx.callbacks:
                        ctx.callbacks[(r.fileno(), conn.fileno())] = \
                            ctx.callbacks[(r.fileno(), readable.fileno())]
            else:
                # This is the part that would be optimized by
                # splice or sendfile
                data = readable.recv(2048)
                if data:
                    for w in ctx.writables(readable):
                        # TODO: trigger callback
                        if w in writables:
                            w.send(data)
                        else:
                            pass # uhh, TODO: buffer data to unwritables

def init():
    ctx = _context()
    ctx.thread = threading.Thread(target=_context_loop, args=(ctx,))
    ctx.thread.start()
    return ctx

def term(ctx):
    ctx.active = False
    ctx.thread.join()

def join(ctx, fda, fdb, flags=0, callback=None):
    def _join(from_, to, callback=None):
        if from_.fileno() not in ctx.joins:
            ctx.joins[from_.fileno()] = ([], [])
        if to.fileno() not in ctx.joins:
            ctx.joins[to.fileno()] = ([], [])
        ctx.joins[from_.fileno()][1].append(to)
        ctx.joins[to.fileno()][0].append(from_)
        if callable(callback):
            ctx.callbacks[(from_.fileno(), to.fileno())] = callback
    
    fda.setblocking(0)
    fdb.setblocking(0)
    if flags & HALFDUPLEX:
        _join(fda, fdb, callback)
    else:
        _join(fda, fdb, callback)
        _join(fdb, fda, callback)

def unjoin(ctx, fda, fdb, flags=0):
    def _unjoin(from_, to):
        ctx.joins[from_.fileno()][1].remove(to)
        ctx.joins[to.fileno()][0].remove(from_)
        if (from_.fileno(), to.fileno()) in ctx.callbacks:
            ctx.callbacks.pop((from_.fileno(), to.fileno()))

    if flags & HALFDUPLEX:
        _unjoin(fda, fdb)
    else:
        _unjoin(fda, fdb)
        _unjoin(fdb, fda)
