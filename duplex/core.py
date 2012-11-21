import collections
import os
import threading

from tornado.ioloop import IOLoop

READ_SIZE = 1024

# Flags used in public API
HALFDUPLEX = 1
NOCLOSE = 2

class LoopThread(object):
    instance = None

    _pipe = collections.namedtuple('pipe', [
        'to_fd',
        'transform',
        'no_close',
        'write_buffer',])

    def __init__(self):
        self._loop = None
        self._thread = threading.Thread(target=self._run)
        self._thread.start()

    def _run(self):
        self._loop = IOLoop.instance()
        self._loop.start()

    def _create_pipe(self, from_, to, transform, no_close):
        pipe = self._pipe(
            to_fd           = to.fileno(),
            transform       = transform,
            no_close        = no_close,
            write_buffer    = bytearray(),)

        def _ondata(sock, fd, events):
            if len(pipe.write_buffer) == 0:
                bytes = os.read(fd, READ_SIZE)
                if len(bytes) < 1:
                    self._loop.remove_handler(fd)
                    os.close(fd)
                    if not pipe.no_close:
                        os.close(pipe.to_fd)
                    return

                if callable(pipe.transform):
                    bytes = pipe.transform(bytes)

                pipe.write_buffer.extend(bytes)

            bytes_written = os.write(pipe.to_fd, pipe.write_buffer)
            pipe.write_buffer[:bytes_written] = ''

        self._loop.add_handler(
            from_.fileno(), _ondata, self._loop.READ)

    def shutdown(self):
        if hasattr(self._loop, 'stop'):
            self._loop.stop()
        self._thread.join()

    def join(self, a, b, transform=None, no_close=False, half_duplex=False):
        self._create_pipe(a, b, transform, no_close)
        if not half_duplex:
            self._create_pipe(b, a, transform, no_close)

    def unjoin(self, a, b):
        self._loop.remove_handler(a.fileno())
        self._loop.remove_handler(b.fileno())

def shutdown():
    if LoopThread.instance:
        LoopThread.instance.shutdown()

def join(a, b, flags=0, transform=None):
    no_close = flags & NOCLOSE
    half_duplex = flags & HALFDUPLEX

    if LoopThread.instance is None:
        LoopThread.instance = LoopThread()

    LoopThread.instance.join(a, b, transform, no_close, half_duplex)

def unjoin(a, b):
    if LoopThread.instance:
        LoopThread.instance.unjoin(a, b)
