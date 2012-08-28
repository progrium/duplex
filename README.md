# Duplex: Managed Socket API (early phase 1)

Duplex is a library to augment socket programming. It's primary feature
right now is socket joining. Socket joining makes building fast TCP "plumbing"
(proxies, gateways, routers, and more) very easy.

Currently, this is a proof of concept where the functionality is
implemented in Python, however the idea is to re-implement it in C and
expose an API that makes it easy to provide bindings in many languages. 

Warning: The idea is still evolving and isn't quite clear. It was originally
about one thing but has evolved into maybe a new sort of "managed socket
service" API.

## Phase 1: Socket joining

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
primitive of Duplex:

	duplex.join(SocketA, SocketB)

Everything is taken care of for you. You can unjoin them at any time.
You can join them at any time. In fact, that's a feature so you can
setup your socket/connection however you like, handle handshakes or
whatever, then join the streams. No matter what environment you use
Duplex, you get this simple API and everything is taken care of for
you, no matter if you're using an event loop, multithreading, greenlets. 

What's more, we optimize it on two levels: 1) the streams are joined by
a very tight C loop, 2) we use splice or sendfile to join the sockets
with "zero-copy", meaning we avoid moving data across the kernel/app
boundary. Now you can write custom proxies as fast as HAProxy in
the language of your choice. 

If we can make it nearly free to join sockets, you'd use it for things
you never would have before...

See the examples directory for ideas.

## Phase 2: More powerful, higher level socket API

If this works, then we start exposing more kernel tricks (sharing
sockets across processes, sendfile, splice, tee, etc) and higher level
functionality into a simple API that looks like maybe BSD sockets "v2".

	socket = duplex.Socket()
	socket.connect("tcp://domain.com:5050")
	# or 
	socket.bind("tcp://0.0.0.0:9000") # auto listen()
	# calling connect or bind again might create 
	# another socket and auto join it to this one.
	socket.accept() # pulls from queue of auto accepted connections
	socket.join(othersocket) # socket joining built in
	socket.send() # always uses sendall, unless non blocking
	socket.sendfile(fd) 
	socket.close()
	
Mostly conveniences on top of BSD sockets. But we can expose more. Also,
these are "managed sockets"... meaning they might have operations (such
as socket joining) going on behind the scenes. This opens the door for
phase 3.


## Phase 3: Messaging layer / toolkit

Messaging on the socket. Generalized low level messaging. Primarily
about message frames. Pluggable codecs for encoding/decoding frames or
payloads of various message formats: websocket, line-based or generic
delimiter, zeromq, chunked transfer or generic length-prefixed, etc. 

	socket.setcodec(duplex.WEBSOCKET) # sets framing encode/decode
	socket.route() # similar to join, but uses put(get()), ie uses codec
	socket.get() # pull a message off using codec
	socket.put() # put a message in using codec

Some plumbing is about messages. If you wanted to make a fast websocket
to TCP gateway, it might look like this:

	# some http/websocket server did handshake to setup connection,
	# then hands us the raw socket
	def handle_websocket(socket):
	  socket.setcodec(duplex.WEBSOCKET, payload=True) # payloads not frames
	  tcpsock = socket.create_connection((some_host, some_port))
	  # tcpsock gets no codec so remains a stream. ie put is just send
	  socket.route(tcpsock) # pulls websocket messages into tcpsock send

Pluggable, fast messaging right on the socket!

Megasock would come with various message codecs in C so you get efficient
messaging support in all languages. Keep in mind this is not about
handshakes or actually implementing the protocol, it's about the stream
of message frames or payloads, nothing more.
