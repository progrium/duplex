from ctypes import *
import atexit

global dpx
dpx = CDLL('libdpx.so')

dpx.dpx_init()

def cleanup_dpx():
    dpx.dpx_cleanup()

atexit.register(cleanup_dpx)

__all__ = ["channel", "error", "frame", "peer"]
