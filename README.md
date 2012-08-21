# Megasock proof of concept

Megasock is a socket toolkit that makes building fast TCP "plumbing"
(proxies, gateways, routers, and more) very easy. This is a proof of concept
where the functionality is implemented in Python, however the idea is to
re-implement it in C and expose an API that makes it easy to provide
bindings in many languages. 

Warning: The idea is still evolving and isn't quite clear. It was originally
about one thing but has evolved into maybe a new sort of "managed socket
service" API.

## Core idea: Socket joining

Almost every bit of TCP plumbing (that is, anything in between an
application client and application server) involves this pattern
represented in pseudo-code:

	Thread:
	  while True:
	    SocketA.send(SocketB.recv())
	Thread:
	  while True:
	    SocketB.send(SocketA.recv())

This is how you write proxies and, well, pretty much everything else is
a variation on a proxy (routers, balancers, gateways, tunnels, etc). What you do
around this pattern is what's interesting and is specific to your bit of
plumbing, whether it's a health-based load balancer, or a secure tunnel,
or a port forwarder. The idea is this pattern should be optimized and
made available to all languages. This is socket joining, the primary
primitive of Megasock:

	MegaSock_Join(SocketA, SocketB)

Everything is taken care of for you. You can unjoin them at any time.
You can join them at any time. In fact, that's a feature so you can
setup your socket/connection however you like, handle handshakes or
whatever, then join the streams. No matter what environment you use
Megasock, you get this simple API and everything is taken care of for
you, no matter if you're using an event loop, multithreading, greenlets. 

What's more, we optimize it on two levels: 1) the streams are joined by
a very tight C loop, 2) we use splice or sendfile to join the sockets
with "zero-copy", meaning we avoid moving data across the kernel/app
boundary. Now you can write custom proxies as fast as HAProxy in
the language of your choice. 

If we can make it nearly free to join sockets, you'd use it for things
you never would have before...

## Building on this primitive

Looking at our examples, you can see some interesting patterns and
applications you can build with socket joining. 

Write more...

## Another core stream primitive

	inspect() # good api for efficiently peeking into a sockets buffer

## Possible:

New slightly higher level, faster socket primitive? ZMQ-inspired...

	socket = PowerSocket() # just trying on different names
	socket.connect("tcp://domain.com:5050")
	# or 
	socket.bind("tcp://0.0.0.0:9000") # auto listen()
	# calling connect or bind again might create 
	# another socket and auto join it to this one.
	socket.accept() # pulls from queue of auto accepted connections
	socket.join(othersocket) # socket joining built in
	socket.readline() # auto-filelike object? maybe not, see below
	socket.send() # always uses sendall
	socket.close() # proper close and shutdown
	
	# see next section
	socket.setmsgcodec(WEBSOCKET | ZMQ | HTTPCHUNKED | NEWLINE)
	socket.get()
	socket.put()
	socket.route(anothersocket) # like join but for messages

## Messaging layer!

A whole new level. Only about message framing. Pluggable codecs for
encoding/decoding frames or payloads of various message formats:
websocket, line-based or generic delimiter, zeromq, chunked transfer or
generic length-prefixed, etc. 

	msg_codec() # sets framing encode/decode
	msg_route() # similar to join, but uses put(get()), ie uses codec
	msg_get() # pull a message off using codec
	msg_put() # put a message in using codec
	msg_queue() # fast queue primitive that can be treated like a socket

Some plumbing is about messages. If you wanted to make a fast websocket
to TCP gateway, it might look like this:

	# some http/websocket server did handshake to setup connection,
	# then hands us the raw socket
	def handle_websocket(socket):
	  socket.setmsgcode(WEBSOCKET, payload=True) # payloads not frames
	  tcpsock = socket.create_connection((some_host, some_port))
	  # tcpsock gets no codec so remains a stream. ie put is just send
	  socket.route(tcpsock) # pulls websocket messages into tcpsock send

Messaging is also useful for setting up multiplexing etc. Pluggable
messaging right on the socket!

Megasock would come with various message codecs in C so you get efficient
messaging support in all languages. Keep in mind this is not about
handshakes or actually implementing the protocol, it's about the stream
of message frames or payloads, nothing more.
