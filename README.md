# Duplex Prototype

Duplex combines ideas from ZeroMQ, Twitter Finagle, and BERT-RPC for a simple, efficient, extensible application communications protocol and library.

Here are some novel features of Duplex RPC:
 * Built around message streams for RPC input (arguments), output (return value), and errors
 * Payload agnostic. Serialize data however you like, or with the default codecs
 * ZeroMQ-style "sockets" with builtin queues, round-robin balancing, and retry

Although heavily inpsired by ZeroMQ and nanomsg, here are reasons it doesn't use either:
 * Stream-oriented request/reply cannot be achieved without multiple socket types, each on their own port
 * Message patterns of different socket types would be implemented at a higher level, making them useless
 * No ability to reverse REQ/REP roles (say, for callbacks) without new sockets on new ports

In short, the messaging abstractions work against the goals of Duplex. However there are many great features that Duplex emulates, mostly to employ the overall "smart application/edge, dumb switch/network" messaging philosophy, such as building in connection pools, edge queuing/retrying, and optimizing for high-throughput usage.

Then combine this with the powerful RPC semantics of BERT-RPC, built around a Unix-inpsired "stream model", and package it in a well-defined C library ready for easy bindings in your favorite language... then you have a powerful communications primitive for service-oriented systems. 

The roadmap of this early project looks like this:
 * Prototype in Go
 * Validate by using in Flynn components
 * Document protocols
 * Port to C (using the Go-like libtask)
 * Write bindings and popularize!

## How it works

Duplex is made for RPC, but at its core is actually a powerful system for managing channels of streams between Duplex peers.

### Sockets

First of all, Duplex has a concept of a socket like ZeroMQ that is not actually a single TCP socket, but actually a hub of connections. A Duplex socket can both bind to one or more ports listening for connections, or connect out to one or more remote sockets. Like ZeroMQ, Duplex decouples the connection topology from the messaging topology. At the end of the day, a Duplex socket is a peer. 

In addition to bind and connect, an auto method on sockets lets you specify a lazy mechanism to connect to hosts. This lets you tie Duplex into service discovery systems.

### Channels

So what can you do with these sockets? Once you have a socket with connections, you can open a named channel. A channel consists of three distinct streams: an input stream, an output stream, and an error stream. It's very similar to the Unix pipeline model. When you open a channel on a socket, it will open that channel with one of the peer connections. The selection of the peer is simply round robin. Once a channel is open, all streams on that channel are between you and that peer connection.

### Streams

Unlike Unix or TCP byte streams, Duplex streams are message streams. A message stream can consistent of zero, one, or many messages and then is terminated by a sort of EOF message. There are three types of streams for any channel: input, output, and error. Input and output is unidirectional, however (unlike Unix streams) error is bidirectional. The usual interaction between peers on a channel is: some input is streamed by the peer that opened the channel, the remote peer streams back output, or potentially errors. Now, remember a stream could consistent of a single message, and in that case you have a pretty standard request / reply model. You might begin to see how you can build RPC on top of this.

### Codecs

Before we get to RPC, we should talk about what's in the messages. As far as the Duplex protocol is concerned, the payload of messages is just a bag of bytes. However, the Duplex sockets let you register codecs to serialize and deserialize these payloads for you. In this way, Duplex doesn't care if you use Protocol Buffers, MessagePack, or JSON, so long as both peers have the codec registered. In fact, you can have multiple codecs registered so that some clients can use Protocol Buffers, and others can use JSON. Duplex leaves payload serialization up to you.

### RPC

RPC really comes in at the end. The names of the channels are used to map to registered functions exposes on a Duplex socket. Depending on the language bindings, a function can look pretty normal, taking some arguments and returning a single return value. In this case, the input stream consists of a single message of the serialized arguments followed by an EOF, then the ouput stream consists of the serialized result followed by an EOF. But what's powerful is when you want to stream multiple results back, or having an argument of streaming data in. 

Because Duplex sockets are peers, there is no technical distinction of client or server. What this means is that a "server" peer can make RPC calls back to the peers connected to it, using the same API. This allows for advanced RPC patterns like callbacks.

### Queues

Lastly, all operations are asynchronous and queued locally in memory just like ZeroMQ. As such, you can start calls before connecting up any peers, or add peers and it automatically balances across them. This gives you quite a bit of flexibility, but it also allows for optimizations such as intelligent batch sends. 

### Extensibility

Besides codecs, you can extend Duplex frames with extra metadata in the same way you can HTTP requests using key-value headers. This metadata can be used by your own custom infrastructure for advanced routing or tracing, without affecting the actual RPC payloads. Headers are also used for advanced RPC semantics borrowed from BERT-RPC, such as caching.

## License

BSD