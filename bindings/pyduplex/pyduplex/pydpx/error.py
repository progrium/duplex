
class DpxError(Exception):

    def __init__(self, code):
        message = "dpx: something went horribly wrong"

        if code == -50:
            message = "dpx: extremely unspecific fatal error occured"
        elif code == 1:
            message = "dpx: object already free'd"
        elif code == 10:
            message = "dpx: channel is already closed"
        elif code == 11:
            message = "dpx: failed to send frame"
        elif code == 20:
            message = "dpx: network failure"
        elif code == 21:
            message = "dpx: not all bytes were sent"
        elif code == 30:
            message = "dpx: peer is already closed"
        elif code == 40:
            message = "dpx: duplex connection closed"

        Exception.__init__(self, message)
