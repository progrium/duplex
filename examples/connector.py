"""Connector example

Here we create a server that listens on a port and joins all connections
that come in on that port. This can be used for two processes to
rendezvous, or for one process to broadcast to many processes. 

 client-->)-connector-(<--client

Try it with:

    $ python examples/connector.py

Then connect:

    $ nc localhost 10000

You connect but nothing happens. Connect with another client:

    $ nc localhost 10000

Talk to each other! Two sockets were created and end up connected, but
neither was set up to listen. Well we can also use this to broadcast.
Connect more clients!

    $ nc localhost 10000

Everybody will get all streams. This could be modified to give each
connecting pair their own private port by building a small protocol to
provision ports to incoming connections and having them discover their
private port from the main server port.

"""
import socket
import sys
sys.path.append(".")
sys.path.append("..")

import duplex


connections = []

try:
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.bind(("0.0.0.0", 10000))
    server.listen(1024)

    while True:
        new_conn, address = server.accept()
        for conn in connections:
            duplex.join(new_conn, conn)
finally:
    duplex.shutdown()


