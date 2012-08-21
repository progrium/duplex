"""Simple join of socket pairs

Here we create two socket pairs. Each pair has a "left" and a "right"
socket. It would sort of look like this:

    pair1_left <--> pair1_right
    pair2_left <--> pair2_right

Then we join the "right" socket of one pair to the "left" of the
other. This effective joins them all together in a virtual pairing:

    pair1_left <--> pair1_right <-join-> pair2_left <--> pair2_right

This looks like a lot, but since the joined sockets are managed by
megasock, you don't have to worry about them. In the end, this is what
we accomplish:

    pair1_left <--> pair2_right

"""
import socket
import sys
sys.path.append(".")
sys.path.append("..")

import megasock

ctx = megasock.init()

try:
    pair1_left, pair1_right = socket.socketpair(socket.AF_UNIX, socket.SOCK_STREAM)
    pair2_left, pair2_right = socket.socketpair(socket.AF_UNIX, socket.SOCK_STREAM)

    megasock.join(ctx, pair1_right, pair2_left)

    pair1_left.send("Hello from pair1")
    print "pair2 got:", pair2_right.recv(1024)

    pair2_right.send("Hello from pair2")
    print "pair1 got:", pair1_left.recv(1024)

finally:
    megasock.term(ctx)

