from ctypes import *
from pyduplex.pydpx.error import DpxError

dpx = CDLL('libdpx.so')

class Peer(object):

    def __init__(self):
        self.peer = dpx.dpx_peer_new()

    def free(self):
        """free the peer"""
        dpx.dpx_peer_close(self.peer)
        dpx.dpx_peer_free(self.peer)

    def open(self, method):
        """open a connection with the specified method"""
        chan = dpx.dpx_peer_open(self.peer, c_char_p(method))
        if chan is None:
            return None
        return Channel(chan)

    def accept(self):
        """accept a new connection to the peer"""
        chan = dpx.dpx_peer_accept(self.peer)
        if chan is None:
            return None
        return Channel(chan)

    def close(self):
        """close the peer"""
        res = dpx.dpx_peer_close(self.peer)
        if res.value != 0:
            raise DpxError(res.value)

    def connect(self, host, port):
        """connect to another peer"""
        res = dpx.dpx_peer_connect(self.peer, c_char_p(host), c_int(port))
        if res.value != 0:
            raise DpxError(res.value)

    def bind(self, host, port):
        """bind and listen on a specific port"""
        res = dpx.dpx_peer_bind(self.peer, c_char_p(host), c_int(port))
        if res.value != 0:
            raise DpxError(res.value)
