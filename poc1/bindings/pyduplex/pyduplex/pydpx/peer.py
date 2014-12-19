from ctypes import *
from pyduplex.pydpx.error import DpxError
from pyduplex.pydpx.channel import Channel

dpx = CDLL('libdpx.so')

class Peer(object):

    def __init__(self):
        dpn = dpx.dpx_peer_new
        dpn.argtypes = []
        dpn.restype = c_void_p
        self.peer = dpn()

    def free(self):
        """free the peer"""
        dpc = dpx.dpx_peer_close
        dpc.argtypes = [c_void_p]
        dpc(self.peer)

        dpf = dpx.dpx_peer_free
        dpf.argtypes = [c_void_p]
        dpf(self.peer)

    def open(self, method):
        """open a connection with the specified method"""
        dpo = dpx.dpx_peer_open
        dpo.argtypes = [c_void_p, c_char_p]
        dpo.restype = c_void_p

        chan = dpo(self.peer, method)
        if chan is None:
            return None
        return Channel(chan)

    def accept(self):
        """accept a new connection to the peer"""
        dpa = dpx.dpx_peer_accept
        dpa.argtypes = [c_void_p]
        dpa.restype = c_void_p

        chan = dpa(self.peer)
        if chan is None:
            return None
        return Channel(chan)

    def close(self):
        """close the peer"""
        dpc = dpx.dpx_peer_close
        dpc.argtypes = [c_void_p]
        dpc.restype = c_int

        res = dpc(self.peer)
        if res != 0:
            raise DpxError(res)

    def connect(self, host, port):
        """connect to another peer"""
        dpc = dpx.dpx_peer_connect
        dpc.argtypes = [c_void_p, c_char_p, c_int]
        dpc.restype = c_int

        res = dpc(self.peer, host, port)
        if res != 0:
            raise DpxError(res)

    def bind(self, host, port):
        """bind and listen on a specific port"""
        dpb = dpx.dpx_peer_bind
        dpb.argtypes = [c_void_p, c_char_p, c_int]
        dpb.restype = c_int

        res = dpb(self.peer, host, port)
        if res != 0:
            raise DpxError(res)
