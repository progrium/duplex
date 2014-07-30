import pytest

from pyduplex.pydpx import *
from threading import Thread
from Queue import Queue

@pytest.fixture
def basic():
    return

def test_peer_frame_send_receive(basic):
    s1 = peer.Peer()
    s1.bind('127.0.0.1', 9878)
    s2 = peer.Peer()
    s2.connect('127.0.0.1', 9878)

    client_chan = s1.open('foobar')
    server_chan = s2.accept()

    assert client_chan.method() == server_chan.method()

    client_input = frame.Frame()
    client_input.payload = bytearray([1,2,3])
    client_input.last = True

    client_chan.send(client_input)

    server_input = server_chan.receive()
    assert server_input.payload == client_input.payload

    server_output = frame.Frame()
    server_output.payload = bytearray([3,2,1])
    server_output.last = True

    server_chan.send(server_output)

    client_output = client_chan.receive()
    assert client_output.payload == server_output.payload

    s1.close()
    s2.close()

    s1.free()
    s2.free()

    # FIXME HOW DO WE CLEANUP THESE CHANNELS?!?!?!
    # server_chan.free()
    # client_chan.free()

def call(peer, method, payload):
    channel = peer.open(method)
    request = frame.Frame()
    request.payload = payload
    request.last = True
    channel.send(request)
    response = channel.receive()
    return response.payload

def test_rpc_call(basic):
    server = peer.Peer()
    server.bind('127.0.0.1', 9877)
    client = peer.Peer()
    client.connect('127.0.0.1', 9877)

    def server_func():
        while True:
            chan = server.accept()
            if chan is None:
                return

            if chan.method() == 'foo':
                recv = chan.receive()
                resp = frame.Frame()
                payload = recv.payload
                payload.reverse()
                resp.payload = payload
                chan.send(resp)

    server_thread = Thread(target=server_func)
    server_thread.start()

    response = call(client, 'foo', bytearray([1,2,3]))
    assert response == bytearray([3,2,1])

    server.close()
    client.close()

    server_thread.join()

@pytest.fixture
def advanced():
    return

def test_round_robin_async(advanced):
    client = peer.Peer()
    client.connect('127.0.0.1', 9876)
    client.connect('127.0.0.1', 9875)
    client.bind('127.0.0.1', 9874)

    server1 = peer.Peer()
    server1.bind('127.0.0.1', 9876)

    server2 = peer.Peer()
    server2.bind('127.0.0.1', 9875)

    server3 = peer.Peer()
    server3.connect('127.0.0.1', 9874)

    serverQueue = Queue()

    def server_func(peer, id):
        while True:
            chan = peer.accept()
            if chan is None:
                return

            if chan.method() == 'foo':
                serverQueue.put(id)
                recv = chan.receive()
                resp = frame.Frame()
                payload = recv.payload
                payload.reverse()
                resp.payload = payload
                chan.send(resp)

    server1_thread = Thread(target=server_func, args=(server1, 1))
    server1_thread.start()

    server2_thread = Thread(target=server_func, args=(server2, 2))
    server2_thread.start()

    server3_thread = Thread(target=server_func, args=(server3, 3))
    server3_thread.start()

    # wait for all servers to connect
    import time
    time.sleep(1)
    del time

    response = call(client, 'foo', bytearray([1,2,3]))
    assert response == bytearray([3,2,1])

    response = call(client, 'foo', bytearray([1,2,3]))
    assert response == bytearray([3,2,1])

    response = call(client, 'foo', bytearray([1,2,3]))
    assert response == bytearray([3,2,1])

    response = call(client, 'foo', bytearray([1,2,3]))
    assert response == bytearray([3,2,1])

    responses = ''
    for i in range(4):
        responses = '%s%d' % (responses, serverQueue.get())

    assert "1" in responses
    assert "2" in responses
    assert "3" in responses

    server1.close()
    server2.close()
    server3.close()
    client.close()

    server1_thread.join()
    server2_thread.join()
    server3_thread.join()

def test_async_messaging(advanced):
    client = peer.Peer()
    client.connect('127.0.0.1', 9873)

    serverQueue = Queue()

    def client_call():
        serverQueue.put(call(client, 'foo', bytearray([1,2,3])))

    client_call_thread = Thread(target=client_call)
    client_call_thread.start()

    server = peer.Peer()
    server.bind('127.0.0.1', 9873)

    def server_func():
        while True:
            chan = server.accept()
            if chan is None:
                return

            if chan.method() == 'foo':
                recv = chan.receive()
                resp = frame.Frame()
                payload = recv.payload
                payload.reverse()
                resp.payload = payload
                chan.send(resp)

    server_thread = Thread(target=server_func)
    server_thread.start()

    assert serverQueue.get(True) == bytearray([3,2,1])

    server.close()
    client.close()

    client_call_thread.join()
    server_thread.join()