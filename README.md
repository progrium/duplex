# Duplex [![Coverity Scan Build Status](https://scan.coverity.com/projects/2512/badge.svg)](https://scan.coverity.com/projects/2512)

Duplex is a simple, efficient, extensible application communications protocol and library. It's heavily inspired by ZeroMQ and BERT-RPC, but draws influence from Twitter Finagle, HTTP/2, zerorpc, and Unix pipes.

## Features

 * Does efficient RPC, but also supports streaming output (return values) *and* streaming input
 * Intended to be payload agnostic, supporting both dynamic and static systems. Currently just msgpack.
 * Client and server are the same library exposed as async "Peer" objects
 * Peer objects have built-in connection pool and message queue (like ZeroMQ sockets)
 
In short, Duplex is intended to be a language/serialization agnostic RPC and service-oriented communications layer. It's designed for [brokerless messaging architectures](http://zeromq.org/whitepapers:brokerless) and borrows much of the edge-oriented messaging philosophy of ZeroMQ for scalable, distributed deployments.

## Why not ZeroMQ?

Although heavily inpsired by ZeroMQ and nanomsg, here are reasons it doesn't use either:

 * Stream-oriented request/reply cannot be achieved without multiple socket types, each on their own port
 * Message patterns would be implemented at a higher level, making most socket types useless here
 * No ability to reverse REQ/REP roles (say, for callbacks) without new sockets on new ports
 * Writing multiplexing on top of ZeroMQ adds complexity and mostly ignores its messaging patterns (as seen in zerorpc)

In short, the messaging abstractions work against the goals of Duplex. However there are many great features that Duplex emulates, mostly to employ the overall distributed / edge messaging philosophy, and building in connection pools, reconnect logic, edge queuing, and optimizing for high-throughput async usage.

## Implementations

The low level library is called `libdpx`. The high level library (niceties around your language, except if you're using C) is called `duplex`.

### Reference Implementation (prototype)

Prototyped in Go. It is currently usable. Simply `go get -u github.com/robxu9/duplex/prototype`.

### Core C Implementation (libdpx)

Seems to work (tm).

The basic workflow for using `dpx-c` is the following:

* Initialise a worker context, so that all operations happen on the worker thread: `dpx_context *c = dpx_init();`
* Initialise a peer that will utilise that context: `dpx_peer *p = dpx_peer_new(c);`
* Use happily and/or report lots of bugs.

#### Requirements:

* [libtask](https://github.com/robxu9/duplex/tree/master/vendor/libtask)
* [msgpack-c](https://github.com/msgpack/msgpack-c)
* For testing: [check](http://check.sourceforge.net)

Compile with `make`, test with `make check`, install with `make install`.
(Don't forget to run `ldconfig`; I make that mistake way too often.)

#### Troubles:

* bindings may not cleanup the socket left in `/tmp/dpxc_*`. Not really troublesome, but it can pile up.

### Bindings around libdpx (bindings)

#### go-duplex (high level api for golang)

Uses `go-dpx` to JSON encode objects for sending over to other applications. You must have `libdpx` installed on your system prior to running `go get -u github.com/robxu9/duplex/bindings/go-duplex`.

#### go-dpx (low level api for golang)

Interacts directly with `libdpx`. You must have `libdpx` installed on your system prior to running `go get -u github.com/robxu9/duplex/bindings/go-duplex/dpx`.

#### pydpx (low level api for python2)

Via `ctypes`. Seems to work well. You must have `libdpx` installed on your system prior to running `python setup.py install`.

## How it works

Duplex is made for RPC, but at its core is actually a powerful system for managing channels of streams between Duplex peers.

### Peers

Duplex client and server are encapsulated in a Peer, which looks similar to ZeroMQ sockets. A Duplex peer can both bind to one or more ports listening for connections, or connect out to one or more remote sockets. Like ZeroMQ, Duplex decouples the connection topology from the messaging topology. All operations on a peer object are async, queued, and retried.

In addition to bind and connect, an auto method on sockets lets you specify a lazy mechanism to connect to hosts. This lets you tie Duplex into service discovery systems.

### Channels

Once you have a peer with connections, you can open a named channel. A channel encapsulates two streams: an input stream and an output stream. It's very similar to the Unix pipeline model. When you open a channel on a peer, it will open that channel with one of the remote peer connections. The selection of the peer if there is more than one connected is simply round robin. Once a channel is open, all streams on that channel are between you and that peer connection.

### Streams

Unlike Unix or TCP byte streams, Duplex streams are message streams. A message stream can consistent of zero, one, or many messages and then is terminated by a sort of EOF message. The usual interaction between peers on a channel is: some input is streamed by the peer that opened the channel, the remote peer streams back output, and potentially an error. 

Now, remember a stream could consistent of a single message, and in that case you have a pretty standard request / reply model. You might begin to see how you can build RPC on top of this. However, you can also do Unix-style pipelining/streaming.

### Queues

All message operations are asynchronous and queued locally in memory just like ZeroMQ. As such, you can start calls before connecting up any peers, or add peers and it automatically balances across them. This gives you quite a bit of flexibility, but it also allows for optimizations such as intelligent batch sends. 

### Codecs

As far as the Duplex protocol is concerned, the payload of messages is just a bag of bytes. However, the Duplex peers let you register codecs to serialize and deserialize these payloads for you. In this way, Duplex doesn't care if you use Protocol Buffers, MessagePack, or JSON, so long as both peers have the codec registered. In fact, you can have multiple codecs registered so that some clients can use Protocol Buffers, and others can use JSON. 

Duplex does not concern itself with what types of objects it supports and does not invent a new serialization scheme. Instead, it can let you plugin whatever you want, existing or custom.

**Current PoC implementation just uses msgpack**

### Extensibility

Besides codecs, you can extend Duplex frames with extra metadata in the same way you can HTTP requests using key-value headers. This metadata can be used by your own custom infrastructure for advanced routing or tracing, without affecting the actual RPC payloads. Headers are also used for advanced RPC semantics borrowed from BERT-RPC, such as caching and callbacks.

### RPC

RPC is built on top of all this. The names of the channels are used to map to registered functions exposes on a Duplex socket. Depending on the language bindings, a function can look pretty normal, taking some arguments and returning a single return value. In this case, the input stream consists of a single message of the serialized arguments followed by an EOF, then the ouput stream consists of the serialized result followed by an EOF. But what's powerful is when you want to stream multiple results back, or have an argument that can stream data in. 

Because Duplex peers are symmetrical, there is no technical distinction of client or server. What this means is that a "server" peer can make RPC calls back to the peers connected to it, using the same API. This allows for advanced RPC patterns like callbacks.

### Composability

Like ZeroMQ devices, Duplex provides tools to let you build middleman "proxy" services. Using the metadata of headers, similar to HTTP proxies, you can customize routing and add features like authentication or rate limiting, without touching the RPC payloads. This is somewhat similar to Filters in Twitter Finagle.

One "trick" we've been thinking about in tandem with our service discovery system, is allowing a middleman service to insert itself into the path of RPC connections via service discovery hooks. This would allow you to transparently insert and remove RPC proxy services to log traffic, add tracing, split requests, etc. 

## Thanks to

[Jeff Lindsay](https://github.com/progrium), my awesome mentor at DigitalOcean.

## License

BSD
