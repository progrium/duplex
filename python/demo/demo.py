
import asyncio
import websockets
import duplex

rpc = duplex.RPC("json")

@asyncio.coroutine
def echo(ch):
    obj, _ = yield from ch.recv()
    yield from ch.send(obj)
rpc.register("echo", echo)

@asyncio.coroutine
def do_msgbox(ch):
    text, _ = yield from ch.recv()
    yield from ch.call("msgbox", text, async=True)
rpc.register("doMsgbox", do_msgbox)

@asyncio.coroutine
def server(conn, path):
    peer = yield from rpc.accept(conn)
    yield from peer.route()

start_server = websockets.serve(server, 'localhost', 8001)
asyncio.get_event_loop().run_until_complete(start_server)
asyncio.get_event_loop().run_forever()
