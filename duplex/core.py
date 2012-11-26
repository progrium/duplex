import collections
import fcntl
import os
import threading
from functools import partial

from tornado.ioloop import IOLoop, PeriodicCallback

from duplex.error import Error

READ_BUFFER_SIZE = 4096

_buffers = {}

def _close_fd(fd):
    """Close a file descriptor, ignoring EBADF."""
    if os.isatty(fd):
        return
    try:
        os.close(fd)
    except Error.EBADF:
        pass

def _set_nonblocking(fd):
    flags = fcntl.fcntl(fd, fcntl.F_GETFL)
    fcntl.fcntl(fd, fcntl.F_SETFL, flags | os.O_NONBLOCK)

class LoopThread(IOLoop):
    thread = None
 
    def spinup(self):
        if self.thread is None:
            self.thread = threading.Thread(target=self.run_until_done)
            self.thread.daemon = True
            self.thread.start()

    def run_until_done(self):
        # If the loop ever reaches a point where the only handler is the
        # 'waker', terminate it (we're not a long-running web server, so we
        # get to do this).
        def spindown_check():
            if self.running():
                if self._handlers.keys() == [self._waker.fileno()]:
                    self.stop()
                else:
                    self.add_callback(spindown_check)
        self.add_callback(spindown_check)

        #def debug():
        #    print _buffers
        #p = PeriodicCallback(debug, 2000, self)
        #p.start()

        self.start()
        self.thread = None

    # Now provide better handler methods

    def __init__(self, *args, **kwargs):
        super(self.__class__, self).__init__(*args, **kwargs)
        self._read_handlers = {}
        self._write_handlers = {}

    def nuke_handlers(self, fd):
        """Remove a handler from a loop, ignoring EBADF or KeyError."""
        try:
            self.remove_handler(fd)
        except (KeyError, Error.EBADF):
            pass
        self._read_handlers.pop(fd, None)
        self._write_handlers.pop(fd, None)

    def _ensure_dispatcher(self, fd):
        if fd not in self._handlers:
            self.add_handler(fd, self._handler_dispatch, 
                    self.READ | self.WRITE | self.ERROR)

    def _handler_dispatch(self, fd, events):
        if events & self.ERROR or events & self.READ:
            read_handler = self._read_handlers.get(fd)
            if read_handler:
                read_handler(fd, events)
        if events & self.ERROR or events & self.WRITE:
            write_handler = self._write_handlers.get(fd)
            if write_handler:
                write_handler(fd, events)

    def add_read_handler(self, fd, handler):
        self._ensure_dispatcher(fd)
        self._read_handlers[fd] = handler

    def add_write_handler(self, fd, handler):
        self._ensure_dispatcher(fd)
        self._write_handlers[fd] = handler

    def remove_read_handler(self, fd):
        self._read_handlers.pop(fd, None)

    def remove_write_handler(self, fd):
        self._write_handlers.pop(fd, None)

    # Implement our own singleton methods in case somebody uses 
    # the Tornado IOLoop singleton in their app

    @staticmethod
    def initialized():
        return hasattr(LoopThread, "_instance")

    @staticmethod
    def instance():
        if not hasattr(LoopThread, "_instance"):
            with LoopThread._instance_lock:
                if not hasattr(LoopThread, "_instance"):
                    # New instance after double check
                    LoopThread._instance = LoopThread()
        return LoopThread._instance


def shutdown():
    loop = LoopThread.instance()
    if loop.running():
        loop.stop()
    if loop.thread:
        loop.thread.join()
    loop.close()

def wait():
    loop = LoopThread.instance()
    if loop.thread:
        loop.thread.join()

def join(fd, with_fd):
    pipe(fd, with_fd)
    pipe(with_fd, fd)

def release(fd):
    LoopThread.instance().nuke_handlers(fd)
    for buffers in _buffers:
        buffers.pop(fd, None)
    _buffers.pop(fd, None)

def pipe(fd, to_fd):
    loop = LoopThread.instance()

    # Every output file descriptor gets its own buffer, to begin with.
    if fd not in _buffers:
        _set_nonblocking(fd)
        _buffers[fd] = {} 
    buffers = _buffers[fd]

    if to_fd in buffers:
        return

    _set_nonblocking(to_fd)
    buffers[to_fd] = collections.deque()
    
    def schedule_clean_up_writers():
        for output_fd, output_buffer in buffers.iteritems():
            if not output_buffer:
                loop.nuke_handlers(output_fd)
                _close_fd(output_fd)
            else:
                loop.add_write_handler(output_fd, 
                        partial(writer, terminating=True))

    def clean_up_reader(input_fd, close=False):
        loop.nuke_handlers(input_fd)
        if close:
            del _buffers[input_fd]
            _close_fd(input_fd)

    def reader(fd, events):
        # If there's an error on the input, flush the output buffers, close and
        # clean up the reader, and stop.
        if events & loop.ERROR:
            schedule_clean_up_writers()
            clean_up_reader(fd, close=True)
            return

        # If there are no file descriptors to write to any more, stop, but
        # don't close the input.
        if not buffers:
            clean_up_reader(fd, close=False)
            return

        # The loop is necessary for errors like EAGAIN and EINTR.
        while True:
            try:
                data = os.read(fd, READ_BUFFER_SIZE)
            except (Error.EAGAIN, Error.EINTR):
                continue
            except (Error.EPIPE, Error.ECONNRESET, Error.EIO):
                schedule_clean_up_writers()
                clean_up_reader(fd, close=True)
                return
            break

        # The source of the data for the input FD has been closed.
        if not data:
            schedule_clean_up_writers()
            clean_up_reader(fd, close=True)
            return

        # Put the chunk of data in the buffer of every registered output.
        # If an output FD has been closed, remove it from the list of buffers.
        bad_fds = []
        for output_fd, buffer in buffers.iteritems():
            buffer.appendleft(data)
            try:
                loop.add_write_handler(output_fd, writer)
            except Error.EBADF:
                bad_fds.append(output_fd)
        for bad_fd in bad_fds:
            del buffers[bad_fd]

    def writer(fd, events, terminating=False):
        if events & loop.ERROR:
            loop.nuke_handlers(fd)
            del buffers[fd]
            return

        # There's no input -- unschedule the writer, it'll be rescheduled again
        # when there's something for it to write.
        if not buffers[fd]:
            loop.remove_write_handler(fd)
            if terminating:
                _close_fd(fd)
            return

        data = buffers[fd].pop()
        while True:
            try:
                os.write(fd, data)
            except (Error.EPIPE, Error.ECONNRESET, Error.EIO, Error.EBADF):
                del buffers[fd]
                loop.nuke_handlers(fd)
            except (Error.EAGAIN, Error.EINTR):
                continue
            break

    # Start with just the reader.
    loop.add_read_handler(fd, reader)
    loop.spinup()

