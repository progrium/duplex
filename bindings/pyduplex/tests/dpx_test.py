import pytest

from pyduplex.pydpx import *

@pytest.fixture
def basic():
    return

def test_peer_frame_send_receive(basic):
    s1 = peer.Peer()
    s1.bind('127.0.0.1', 9876)
    s2 = peer.Peer()
    s2.bind('127.0.0.1', 9876)

    client_chan = s1.open('foobar')
    server_chan = s2.accept()

    assert client_chan.method() == server_chan.method()

    client_input = Frame()
    client_input.payload = bytearray([1,2,3])
    client_input.last = True

    client_chan.send(client_input)

    server_input = server_chan.receive()
    assert server_input.payload == client_input.payload

    server_output = Frame()
    server_output.payload = bytearray([3,2,1])
    server_output.last = True

    server_chan.send(server_output)

    client_output = client_chan.receive()
    assert client_output.payload == server_output.payload

    server_chan.free()
    client_chan.free()

    s1.close()
    s2.close()

    s1.free()
    s2.free()
