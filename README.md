# Simplex RPC

Simple but powerful RPC layer.

 * Transport agnostic
 * Serialization agnostic
 * Language agnostic
 * Peer based, not client-server
 * Bi-directional, callbacks
 * Streaming results and arguments
 * Extensible with middleware
 * Simple protocol and API guide

Concerns it leaves out:

 * Framed transport
 * Serialization format
 * Auth mechanisms
 * Everything else

Middleware hooks, for:

 * Tracing
 * Auth
 * Stats
 * Policy

sendobj(conn, outgoing)
  send (return)
  error (exception, etc)
  drop/ignore
recvobj(conn, incoming)
  handle (return)
  drop/ignore
  send back error

## Tour

Simplex is an RPC protocol designed for dynamic and some statically-typed languages that avoids conflating object serialization with RPC semantics. It lets you pick how to marshal objects, whether with JSON, msgpack, protobuf, BERT, BSON, or anything custom.

```javascript
// rpc setup using json
```

```golang
// rpc setup using gob
```

While that alone is somehow already revolutionary for RPC protocols, it also has a bi-directional peer model, letting either side of the connection call or provide invocable service methods. This means implementations are both client and server, so you never have to worry about only having a client or only having a server library in your language. However, it also allows for flexible connection topologies (server connecting to clients), callbacks, and plugin architectures.

```python
# server with methods connects to client
```

```javascript
// method gets a callback and calls back to the client
```

If that weren't enough, methods can stream multiple results *and* accept multiple streaming arguments, letting you (ab)use RPC for bi-directional object streaming. Among other patterns, this lets you implement real-time data synchronization, connection tunneling, and interactive consoles, all with the same RPC mechanism as simple remote method calls.

```golang
// calls an interactive method, attaches to terminal
```

```ruby
# call to subscribe to updates
```

Along with a simple protocol spec not much more complex than JSON-RPC, Simplex has an API guide that can be used for easy and consistent implementations in various languages.

The API design has a simple framed transport interface. This means out of the box you can expect to use any transport that takes care of framing, for example WebSockets, UDP, ZeroMQ, AMQP. Wrapping streaming transports such as raw TCP with length-prefix framing lets you use them as well. By focusing on frames and making the API transport agnostic, as well as being serialization agnostic, implementations are very simple with no major dependencies.

The protocol and API are also designed to be extensible, providing a middleware mechanism that lets you add tracing, authentication, policy, transactions, and more. This allows Simplex to remain simple.

# TODO

 * add peer ext to golang
 * add tests for peer ext
