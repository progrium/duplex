"""Connector example

Here we create a server that listens on a port for two connections,
then connects them as if they connected directly to each other.

 client-->)-connector-(<--client

Try it with:

    $ python examples/connector.py 9000

Then connect:

    $ nc localhost 9000

You connect but nothing happens. Connect with another client:

    $ nc localhost 9000

Talk to each other! Two sockets were created and ended up connected, but
neither was set up to listen. Magic plumping! 

"""
import socket
import sys
sys.path.append(".")
sys.path.append("..")

import duplex


server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server.bind(("0.0.0.0", int(sys.argv[1])))
server.listen(1024)

first, address = server.accept()
second, address = server.accept()

duplex.join(first, second)
duplex.wait()

