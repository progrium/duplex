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

 ## License
 
 BSD