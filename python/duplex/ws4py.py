import duplex
import duplex.sync
import queue
import threading
from ws4py.client.threadedclient import WebSocketClient

def Client(url):
    event = threading.Event()
    conn = Conn(url, event)
    conn.connect()
    event.wait()
    return conn


class Conn(WebSocketClient):
    def __init__(self, url, onopened):
        super().__init__(url)
        self.onopened = onopened
        self.inbox = queue.Queue()

    def opened(self):
        self.onopened.set()

    def received_message(self, m):
        self.inbox.put(m.data.decode("utf-8"))

    def recv(self):
        return self.inbox.get()

    def close(self, code=1000, reason=""):
        self.inbox.put("")
        super().close(code, reason)
