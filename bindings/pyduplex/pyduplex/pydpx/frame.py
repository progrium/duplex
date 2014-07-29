from ctypes import *

class Frame(object):

    def __init__(self, frame):
        self.headers = {}
        self.last = (frame.last != 0)
        payload = []

        for i in range(c_ulong(frame.payloadSize).value):
            payload.append(frame.payload[i])

        self.payload = bytearray(payload)

        if frame.method is not None:
            self.method = c_char_p(frame.method)

        if frame.error is not None:
            self.error = c_char_p(frame.error)

        def frame_iter_helper(ptr, k, v):
            self.headers[c_char_p(k).value] = c_char_p(v).value

        CMPFUNC = CFUNCTYPE(None, c_void_p, c_char_p, c_char_p)
        callback = CMPFUNC(frame_iter_helper)

        dpx.dpx_frame_header_iter(frame, callback, None)

    def to_c(self):
        cframe = dpx.dpx_frame_new(None)
        cframe.method = c_char_p(self.method)
        cframe.error = c_char_p(self.error)
        cframe.headers = None

        if frame.Last:
            cframe.last = c_int(1)
        else:
            cframe.last = c_int(0)

        for k in self.headers:
            dpx.dpx_frame_header_add(cframe, c_char_p(k), c_char_p(self.headers[k]))

        if len(self.payload) != 0:
            cframe.payloadSize = c_ulong(len(self.payload))

            libc = CDLL('libc.so.6')
            cframe.payload = libc.malloc(cframe.payloadSize)

            payload = cast(cframe.payload, POINTER(c_byte))

            for i in range(len(self.payload)):
                payload[i] = self.payload[i]

        return cframe
