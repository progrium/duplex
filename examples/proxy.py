"""Proxy example

Here is a simple proxy that will listen on port 10000 and forward any
connections to port 8000. 

    client-->)-proxy--->backend

You can test this with netcat:

    $ nc -l 8000

Then:

    $ python examples/proxy.py

Then:

    $ nc localhost 10000

Type some stuff into STDIN and you'll see it in the original netcat
server/listen session. A really simple (fast) proxy!

"""
import socket
import sys
sys.path.append(".")
sys.path.append("..")

import megasock

ctx = megasock.init()

try:
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.bind(("0.0.0.0", 10000))
    server.listen(1024)

    while True:
        conn, address = server.accept()
        backend = socket.create_connection(("0.0.0.0", 8000))
        megasock.join(ctx, conn, backend)
finally:
    megasock.term(ctx)

