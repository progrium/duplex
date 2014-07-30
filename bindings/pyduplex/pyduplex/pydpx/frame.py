from ctypes import *

dpx = CDLL('libdpx.so')

class CFRAME(Structure):
    _fields_ = [('errCh', c_void_p),
                ('chanRef', c_void_p),
                ('type', c_int),
                ('channel', c_int),
                ('method', c_char_p),
                ('headers', c_void_p),
                ('error', c_char_p),
                ('last', c_int),
                ('payload', POINTER(c_byte)),
                ('payloadSize', c_int)]

class Frame(object):

    def __init__(self):
        self.headers = {}
        self.last = False
        self.payload = bytearray([])
        self.method = ''
        self.error = ''

    @classmethod
    def from_c(cls, frame):
        aframe = frame[0]

        self = cls()
        self.headers = {}
        if aframe.last != 0:
            self.last = True
        else:
            self.last = False

        payload = []

        for i in range(aframe.payloadSize):
            payload.append(aframe.payload[i])

        self.payload = bytearray(payload)

        if aframe.method is not None:
            self.method = aframe.method

        if aframe.error is not None:
            self.error = aframe.error

        def frame_iter_helper(ptr, k, v):
            self.headers[k] = v

        CMPFUNC = CFUNCTYPE(None, c_void_p, c_char_p, c_char_p)
        callback = CMPFUNC(frame_iter_helper)

        dfhi = dpx.dpx_frame_header_iter
        dfhi.argtypes = [POINTER(CFRAME), c_void_p, c_void_p]

        dfhi(frame, callback, None)

        return self

    def to_c(self):
        dfn = dpx.dpx_frame_new
        dfn.argtypes = [c_void_p]
        dfn.restype = POINTER(CFRAME)

        cframeptr = dfn(None)

        cframe = cframeptr[0]

        cframe.method = self.method
        cframe.error = self.error
        cframe.headers = None

        if self.last:
            cframe.last = 1
        else:
            cframe.last = 0

        dfha = dpx.dpx_frame_header_add
        dfha.argtypes = [c_void_p, c_char_p, c_char_p]

        for k in self.headers:
            dfha(cframe, k, self.headers[k])

        if len(self.payload) != 0:
            cframe.payloadSize = len(self.payload)

            libc = CDLL('libc.so.6')
            malloc = libc.malloc
            malloc.argtypes = [c_size_t]
            malloc.restype = c_void_p

            malloced = malloc(len(self.payload))

            payload = cast(malloced, POINTER(c_byte))

            for i in range(len(self.payload)):
                payload[i] = self.payload[i]

            cframe.payload = payload

        return cframeptr
