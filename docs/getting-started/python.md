# Getting Started in Python

## Installing

Duplex requires Python 3.3 or greater.

```
$ pip install duplex
```

## Example

To demonstrate using Duplex, we're going to write a traditional client and server using WebSockets and JSON.

Since Duplex peers are both RPC "clients" and "servers", we use the terms *caller* and *provider* when referring to a peer making a call and a peer providing functions, respectively. Client and server can then be used to refer to the transport topology, with the client establishing the connection and the server accepting the connection.

To keep things simple, in this example we'll stick to the WebSocket client being the only caller, and the WebSocket server being the only provider. Thus resembling your typical RPC client-server scenario.

### Async Server

For the server, we're going to use `asyncio` and a WebSocket library called `websockets`:

```
$ pip install websockets
```
Make a file called `server.py` like this:

```python
import asyncio
import websockets
import duplex

rpc = duplex.RPC("json")

@asyncio.coroutine
def add(args, ch):
  return sum(args)

rpc.register_func("example.add", add)

@asyncio.coroutine
def handle(conn, path):
  peer = yield from rpc.accept(conn, route=False)
  yield from peer.route()

start_server = websockets.serve(handle, 'localhost', 3000)
asyncio.get_event_loop().run_until_complete(start_server)
asyncio.get_event_loop().run_forever()
```
That's a lot of code for a simple add function. But most of it is for setting up the `asyncio` WebSocket server. We'll most likely provide a convenience module to simplify this in the future.

We can run this and it will listen for connections on `localhost:3000`:

```
$ python server.py
```

### Synchronous Client

With the server running, let's use the interactive Python shell to connect and interact with our server instead of writing a script.

Since `asyncio` is great for servers, but not fun to use interactively, we'll need a *different* WebSockets library. We'll use `ws4py` since we have a wrapper for using it from the shell.

```
$ pip install ws4py
```
Now let's jump into the shell by running `python` and importing our modules and making our RPC object:

```python
>>> import duplex
>>> import duplex.ws4py
>>> rpc = duplex.RPC("json")
```

Now we make a WebSocket connection using the builtin wrapper for `ws4py`. Then we give it to our RPC object to perform the Duplex handshake, returning a peer object.

```python
>>> conn = duplex.ws4py.Client("ws://localhost:3000")
>>> peer = duplex.handshake(conn)
```
We're ready to make calls!
```python
>>> peer.call("example.sum", [1,2,3])
6
```
