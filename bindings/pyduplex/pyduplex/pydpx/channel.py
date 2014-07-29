from ctypes import *
from pyduplex.pydpx.error import DpxError
from pyduplex.pydpx.peer import Peer
from pyduplex.pydpx.frame import Frame

class Channel(object):
    def __init__(self, cchan):
        self.chan = cchan

    def free(self):
        dpx.dpx_channel_free(self.chan)

    def method(self):
        return c_char_p(dpx.dpx_channel_method_get(self.chan)).value

    def error(self):
        err = c_int(dpx.dpx_channel_error(self.chan))
        if err.value == 0:
            return None
        return DpxError(err.value)

    def receive(self):
        frame = dpx.dpx_channel_receive_frame(self.chan)
        if frame is None:
            return None

        our_frame = Frame(frame)
        dpx.dpx_frame_free(frame)
        return our_frame

    def send(self, frame):
        err = c_int(dpx.dpx_channel_send_frame(self.chan, frame.to_c()))
        if err.value != 0:
            raise DpxError(err.value)
