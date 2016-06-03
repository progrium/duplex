# Getting Started in Python

## Overview

Here we're going to use Duplex in Python for 3 examples:

 1. A typical RPC service
 1. A streaming RPC service
 1. A callback RPC service

Since Duplex peers are both RPC "clients" and "servers", we use the terms *caller* and *provider* when referring to a peer making a call or a peer providing functions, respectively. Client and server can then be used to refer to the transport topology, where the client establishes the connection and the server listens for connections.

To keep things simple, in these examples we'll stick to the WebSocket client being the only caller, and the WebSocket server being the only provider. Thus resembling your typical RPC client-server scenario.

## Installing

Duplex currently requires Python 3.3 or greater. Our examples will also use several Python libraries including `websockets` and `ws4py`. Install them with `pip3` (assuming you also have Python 2.x. If your primary Python is 3.x, drop the 3 as appropriate):

```
$ pip3 install duplex websockets ws4py
```

## Typical RPC: Sum Service

We're going to first write a traditional client and server using WebSockets and JSON that exposes an `add` function.

### Async Server

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

That's a lot of code for a simple add function. But most of it is for setting up the `asyncio` WebSocket server. We'd love to take a convenience module PR to simplify this in the future.

We can run this and it will listen for connections on `localhost:3000`:

```
$ python3 server.py
```

### Synchronous Interactive Client

With the server running, let's use the interactive Python shell to connect and interact with our server instead of writing a script.

Since `asyncio` is great for servers, but not fun to use interactively, we'll need a *different* WebSockets library. We'll use `ws4py` since we have a wrapper for it made specifically for using Duplex from the shell.

Now let's jump into the shell by running `python3` and importing our modules and making our RPC object:

```python
>>> import duplex
>>> import duplex.ws4py
>>> rpc = duplex.RPC("json", async=False)
```

Now we make a WebSocket connection using the builtin wrapper for `ws4py`. Then we give it to our RPC object to perform the Duplex handshake, returning a peer object.

```python
>>> conn = duplex.ws4py.Client("ws://localhost:3000")
>>> peer = rpc.handshake(conn)
```

We're ready to make calls!

```python
>>> peer.call("example.add", [1,2,3])
6
```

Some things to point out:
 1. JSON could be switched with another codec like MessagePack. Just replace `"json"` with `"msgpack"`.
 1. WebSocket could also be switched out, but we currently only have builtin wrappers for WebSocket.
 1. The core implementation is not only neutral to codec and transport, but important for the Python world is being neutral to threaded and evented models. Here we used both.

So far maybe not so interesting. Let's try the next example.

## Streaming RPC: Counting Service

Next we'll make a streaming service that has a `count` method that streams back incrementing integers every second. Both sides of the connection can stream, but here we're only streaming the reply.

### Async Server

Again, we'll use WebSocket and JSON. In fact, let's just add this streaming method to our existing `server.py`. It should look like this with two new added parts:

```python
import asyncio
import websockets
import duplex

rpc = duplex.RPC("json")

# NEW
@asyncio.coroutine
def count(args, ch):
  n = 0
  while True:
    n += 1
    try:
      yield from ch.send(n, more=True)
      print(n)
      yield from asyncio.sleep(1)
    except Exception as e:
      print(e)
      return

@asyncio.coroutine
def add(args, ch):
  return sum(args)

rpc.register_func("example.add", add)
rpc.register_func("example.count", count) # NEW

@asyncio.coroutine
def handle(conn, path):
  peer = yield from rpc.accept(conn, route=False)
  yield from peer.route()

start_server = websockets.serve(handle, 'localhost', 3000)
asyncio.get_event_loop().run_until_complete(start_server)
asyncio.get_event_loop().run_forever()
```

Similar to `add`, but a little more complex. We ignore the `args` argument as we expect this to be `None`. This is because the method is not invoked until we send something to it. This will make more sense with the client.

Also, this time we're using the `ch` argument to work directly with the channel object. This lets us call `ch.send` multiple times to stream back multiple objects. It's necessary to set `more=True` when streaming.

We can run this and it will listen for connections on `localhost:3000`:

```
$ python3 server.py
```

### Threaded Client

To avoid writing a lot of code, we'll write a synchronous threaded client with `ws4py` like before. This time as a daemon script instead of using the interactive Python shell.

Make a file called `counter.py` like this:

```python
import duplex
import duplex.ws4py

rpc = duplex.RPC("json", async=False)
conn = duplex.ws4py.Client("ws://localhost:3000")
peer = rpc.handshake(conn)
ch = peer.open("example.count")
ch.send(None)
try:
  more = True
  while more:
    num, more = ch.recv()
    print(num)
except KeyboardInterrupt:
  peer.close()

```

Pretty straightforward, but notice we do a `ch.send(None)`. This is because as mentioned before, methods are not invoked until a message is sent on a channel. Since we're not using the higher level `peer.call` API, we manually send on the channel to trigger the method.

Another thing to notice is how we handle streaming responses. `ch.recv()` returns two values: the next object in the stream, and a boolean of whether to expect more values. We use this to loop until the provider is finished, though in this case it never will.

If we run this, we'll get an incrementing number streamed to us until we `Ctrl-C` to close it down:

```
$ python3 counter.py
1
2
3
...
```

As mentioned, you can stream from the caller to the provider. Since there is a channel on both ends, the API is the same. Try modifying the `count` function to expect several objects sent to its channel before starting the counter stream.

At this point the idea of an RPC method or service starts to break down. You could stream objects in either direction, with any order, pattern, or amount. For example, you could return two results instead of streaming many to model a function that returns two values. Working with channels is similar to working with ZeroMQ sockets, letting you build message-based "sub protocols" pretty quickly.

## Callback RPC: Alarm Service

Finally we'll make a callback based service called `alarm` that takes N seconds and a callback, returns `True`, and triggers your callback after N seconds. Both sides of the connection can provide and call callbacks, but here we have the client providing a callback and the server calling it.

### Async Server

Lets keep adding to our `server.py`. It should look like this with two new added parts:

```python
import asyncio
import websockets
import duplex

rpc = duplex.RPC("json")

# NEW
@asyncio.coroutine
def alarm(args, ch):
  seconds = args.get("sec", 0)
  name = args.get("cb")
  if name is not None:
    @asyncio.coroutine
    def cb():
      yield from asyncio.sleep(seconds)
      yield from ch.call(name, wait=False)
    asyncio.ensure_future(cb())
  return True

@asyncio.coroutine
def count(args, ch):
  n = 0
  while True:
    n += 1
    try:
      yield from ch.send(n, more=True)
      print(n)
      yield from asyncio.sleep(1)
    except Exception as e:
      print(e)
      return

@asyncio.coroutine
def add(args, ch):
  return sum(args)

rpc.register_func("example.add", add)
rpc.register_func("example.count", count)
rpc.register_func("example.alarm", alarm) # NEW

@asyncio.coroutine
def handle(conn, path):
  peer = yield from rpc.accept(conn, route=False)
  yield from peer.route()

start_server = websockets.serve(handle, 'localhost', 3000)
asyncio.get_event_loop().run_until_complete(start_server)
asyncio.get_event_loop().run_forever()
```

Besides the `asyncio` nonsense, this probably reads obviously enough. We get the args object and pull the `sec` value from it, and also the `cb` value, which might not be obvious but is a string. Then we make an actual callback function in Python that sleeps for that many seconds and reuses the `ch` object to call the callback function named by the value of `cb`. We schedule that to run and then we return `True`.

Now here is a client script that defines a callback, makes the call and waits for the callback. We'll call it `callback.py`:

```python
import duplex
import duplex.ws4py

rpc = duplex.RPC("json", async=False)

def alarm(args, ch):
  print("ALARM")

conn = duplex.ws4py.Client("ws://localhost:3000")
peer = rpc.handshake(conn)
peer.call("example.alarm", {"sec": 3, "cb": rpc.callback_func(alarm)})
```

If we run the server and then run our `callback.py` script, it should sit for 3 seconds, and sound the alarm, printing "ALARM". The script sits around instead of exiting because of the thread running in the background.

There is nothing terribly magic happening here. The fact either side can make calls to the other is enough to implement callbacks, but we have an extra helper you may have noticed: `rpc.callback_func`. This works like `rpc.register_func` except it registers with a generated name and returns that name as a string. You just need to hand that to the other side and it can call that function back by that name whenever it wants. Although we don't here (hence `wait=False`), it can even return a value.

## What Next

Now you can make Python call into remote Python with callbacks and streaming. Next you should try Duplex in another language and try calling into remote Python from Go or JavaScript.
