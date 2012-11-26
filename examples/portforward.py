"""Port forwarding

Here we have simple port forwarding. The behavior is not all that
different from a simple proxy, except that the proxy is split into two
parts: a server that provides the frontend port to listen for incoming
connections, and a client that connects to the port forward server
waiting for incoming connections and then joins them with connections to
the local server. This gives you a topology where the port server can be
public, and the local server and client can be behind a firewall since
it opens outgoing connections in order to receive incoming connections.

This is like a really simple (and probably faster) version of
localtunnel, except for raw TCP. The major difference is the design:
instead of opening a single connection to the server and multiplexing
all connections through it, here we have the client make several
connections to the server upfront that wait for a connection to come in
on. Having several connections upfront means we don't have to open a new
socket after each incoming connection before we can accept another. 

This model might be an interesting alternative to port sharing in
worker-based servers. No forking is necessary and new workers can come
online without going through and being managed by the master process.

"""
import socket
import select
import sys
sys.path.append(".")
sys.path.append("..")

import duplex

BACKEND_PORT = 9000
FRONTEND_PORT = 10000

CLIENT_POOL_SIZE = 3


try:
    mode = sys.argv[1]
except ValueError:
    mode = None

if mode == "--server":
    server_frontend = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_frontend.bind(("0.0.0.0", FRONTEND_PORT))
    server_frontend.listen(1024)
    server_frontend.setblocking(0)

    server_backend = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_backend.bind(("0.0.0.0", BACKEND_PORT))
    server_backend.listen(1024)
    server_backend.setblocking(0)

    watched_sockets = [server_frontend, server_backend]
    waiting_sockets = []

    while True:
        readables, writables, errs = select.select(
                                        watched_sockets,
                                        watched_sockets,
                                        [], 0)

        for readable in readables:
            if readable == server_backend:
                conn, address = readable.accept()
                waiting_sockets.append(conn)
            elif readable == server_frontend:
                conn, address = readable.accept()
                backend_conn = waiting_sockets.pop()
                duplex.join(conn, backend_conn)
            else:
                pass

elif mode == "--client":
    local_port = sys.argv[2]
    watched_sockets = []

    for i in range(CLIENT_POOL_SIZE):
        conn = socket.create_connection(("0.0.0.0", BACKEND_PORT))
        conn.setblocking(0)
        watched_sockets.append(conn)

    while True:
        readables, writables, errs = select.select(
                                        watched_sockets,
                                        watched_sockets,
                                        [], 0)

        for remote_conn in readables:
            # join "activated" connection to local server
            local_conn = socket.create_connection(("0.0.0.0", local_port))
            watched_sockets.remove(remote_conn)
            duplex.join(remote_conn, local_conn)

            # create pending connection to replace it in the pool
            conn = socket.create_connection(("0.0.0.0", BACKEND_PORT))
            conn.setblocking(0)
            watched_sockets.append(conn)

else:
    print "Invalid mode"

