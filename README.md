# Duplex RPC

Modern full-duplex RPC

 * Serialization and transport agnostic
 * Client and server combined into Peer object
 * Calls and callbacks in either direction
 * Optional streaming of results and arguments
 * Extensible with middleware (soon)
 * Easy to implement protocol and API
 * Ready-to-go implementations
   * Go
   * Python 3
   * JavaScript (Node.js and browser)
   * TODO: Ruby?

## Getting Started

 * [with Python](http://progrium.viewdocs.io/duplex/getting-started/python)
 * with Go
 * with JavaScript

## Tour

### Language and serialization agnostic

Duplex is an RPC protocol designed for dynamic (and some statically-typed) languages that focuses on advanced RPC semantics and not object or frame serialization. This lets you pick how to marshal objects, whether with JSON, msgpack, protobuf, BERT, BSON, or anything custom.

```javascript
// rpc setup using json
```

```golang
// rpc setup using gob
```

### API combines client and server into "peer"

While that alone is somehow already revolutionary for RPC protocols, it also combines client and server into a peer object, letting either side of the connection call or provide invocable service methods. This means you never have to worry about only having a client or only having a server library in your language. It also allows for flexible connection topologies (server calling client functions), callbacks, and plugin architectures.

```python
# server with methods connects to client
```

```javascript
// method gets a callback and calls back to the client
```

### Bi-directional object streaming

Methods can also stream multiple results *and* accept multiple streaming arguments, letting you use Duplex for bi-directional object streaming. Among other patterns, this lets you implement real-time data synchronization, connection tunneling, and interactive consoles, all with the same mechanism as simple remote method calls.

```golang
// calls an interactive method, attaches to terminal
```

```ruby
# call to subscribe to updates
```

### Simple, extensible API and protocol

Duplex has a simple protocol spec not much more complex than JSON-RPC. It also has an API guide that can be used for easy and consistent implementations in various languages.

The protocol and API are also designed to eventually be extensible. Expect a middleware mechanism that lets you add tracing, authentication, policy, transactions, and more. This allows Duplex to remain simple but powerful.

### Transport agnostic, frame interface

The API design has a simple framed transport interface. This means out of the box you can expect to use any transport that takes care of framing, for example WebSockets, UDP, ZeroMQ, AMQP. Wrapping streaming transports such as raw TCP or STDIO with length-prefix or newline framing lets you use them as well. By focusing on frames and making the API transport agnostic, as well as being serialization agnostic, implementations are very simple with no major dependencies.



# TODO

 * document spec / api
 * demo
   * cross language, browser
   * topologies: client-server, server-client, gateway
   * transports: websocket, tcp, UDP
   * codecs: json, msgpack, protobuf
   * streaming: state sync, terminal
   * callbacks: async reply, events, plugins
   * gateway: behind firewall, client to client (browser)
   * patterns: identity, reflection (cli), self docs
   * implementation: code tour, api guide, protocol spec
