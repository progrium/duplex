import core

def _ensure_fd(fd):
    """Ensure an argument is a file descriptor."""
    if not isinstance(fd, int):
        if not hasattr(fd, 'fileno'):
            raise TypeError("Arguments must be file descriptors, or implement fileno()")
        return fd.fileno()
    return fd

def pipe(fd, to_fd):
    core.pipe(_ensure_fd(fd), _ensure_fd(to_fd))

def join(fd, with_fd):
    core.join(_ensure_fd(fd), _ensure_fd(with_fd))

def release(fd):
    core.release(_ensure_fd(fd))

def wait():
    core.wait()

def shutdown():
    core.shutdown()
