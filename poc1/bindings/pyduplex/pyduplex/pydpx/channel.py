from ctypes import *
from pyduplex.pydpx.error import DpxError
from pyduplex.pydpx.frame import CFRAME, Frame

dpx = CDLL('libdpx.so')

class Channel(object):
    def __init__(self, cchan):
        self.chan = cchan

    def free(self):
        dcf = dpx.dpx_channel_free
        dcf.argtypes = [c_void_p]

        dcf(self.chan)

    def method(self):
        dcmg = dpx.dpx_channel_method_get
        dcmg.argtypes = [c_void_p]
        dcmg.restype = c_char_p

        return dcmg(self.chan)

    def error(self):
        dce = dpx.dpx_channel_error
        dce.argtypes = [c_void_p]
        dce.restype = c_int

        err = dce(self.chan)
        if err == 0:
            return None
        return DpxError(err)

    def receive(self):
        dcrf = dpx.dpx_channel_receive_frame
        dcrf.argtypes = [c_void_p]
        dcrf.restype = POINTER(CFRAME)

        frame = dcrf(self.chan)
        if frame is None:
            return None

        our_frame = Frame.from_c(frame)

        dff = dpx.dpx_frame_free
        dff.argtypes = [c_void_p]
        dff(frame)

        return our_frame

    def send(self, frame):
        dcsf = dpx.dpx_channel_send_frame
        dcsf.argtypes = [c_void_p, POINTER(CFRAME)]
        dcsf.restype = c_int

        cframe = frame.to_c()

        err = dcsf(self.chan, cframe)
        if err != 0:
            raise DpxError(err)
