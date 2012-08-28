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
import os
sys.path.append(".")
sys.path.append("..")

import duplex


backend_port = os.environ.get("PORT", 8000)

server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    server.bind(("0.0.0.0", 10000))
    server.listen(1024)

    while True:
        conn, address = server.accept()
        backend = socket.create_connection(("0.0.0.0", backend_port))
        duplex.join(conn, backend)
finally:
    server.close()
    duplex.shutdown()

