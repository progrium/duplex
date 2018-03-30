/// <reference path="../src/duplex.ts" />

export {}

const { duplex } = require('../dist/duplex.js');
const btoa = require('btoa');
const atob = require('atob');

const testErrorCode: number = 1000;
const testErrorMessage: string = "Test message";

class MockConnection {
  constructor() {
    this.sent = [];
    this.closed = false;
    this.pairedWith = null;
    this.onrecv = function() {};
  }

  sent: Array<string>;
  closed: boolean;
  pairedWith: MockConnection;
  onrecv: (frame?: string) => object | void;

  close(): boolean {
    return this.closed = true;
  }

  send(frame: string): object | void {
    this.sent.push(frame);
    if (this.pairedWith) {
      return this.pairedWith.onrecv(frame);
    }
  }

  _recv(frame: string): object | void {
    return this.onrecv(frame);
  }
}

const connectionPair = function(): Array<MockConnection> {
  const conn1 = new MockConnection();
  const conn2 = new MockConnection();
  conn1.pairedWith = conn2;
  conn2.pairedWith = conn1;
  return [conn1, conn2];
};

const peerPair = function(rpc: Duplex.RPC, onready: (p1: Duplex.Peer, p2: Duplex.Peer) => object | void): Duplex.Peer {
  let peer2;
  const [conn1, conn2] = connectionPair();
  const peer1 = rpc.accept(conn1);
  return peer2 = rpc.handshake(conn2, (peer2: Duplex.Peer) => onready(peer1, peer2));
};

const handshake = function(codec: string): string {
  const p = duplex.protocol;
  return `${p.name}/${p.version};${codec}`;
};

const testServices = {
  echo(ch: Duplex.Channel): any {
    return ch.onrecv = (err: object, obj: object) => ch.send(obj);
  },

  generator(ch: Duplex.Channel): any {
    return ch.onrecv = (err: object, count: number) => 
      __range__(1, count, true).map((num) =>
        ch.send({num}, num !== count)
      );
  },

  adder(ch: Duplex.Channel): any {
    let total = 0;
    return ch.onrecv = function(err: object, num: number, more: boolean) {
      total += num;
      if (!more) {
        return ch.send(total);
      }
    };
  },

  error(ch: Duplex.Channel): any {
    return ch.onrecv = () => ch.senderr(testErrorCode, testErrorMessage);
  },

  errorAfter2(ch: Duplex.Channel): any {
    return ch.onrecv = (err: object, count: number) =>
      ((): Array<any> => {
        const result = [];
        for (let num = 1; num <= count; num++) {
          ch.send({num}, num !== count);
          if (num === 2) {
            ch.senderr(testErrorCode, testErrorMessage);
            break;
          } else {
            result.push(undefined);
          }
        }
        return result;
      })()
    ;
  }
};

const b64json = [
  "b64json",
  (obj: object) => btoa(JSON.stringify(obj)),
  (str: string) => JSON.parse(atob(str))
];

describe("duplex RPC", function() {
  it("handshakes", function() {
    const conn = new MockConnection();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    return rpc.handshake(conn, () => expect(conn.sent[0]).toEqual(handshake("json")));
  });

  it("accepts handshakes", function() {
    const conn = new MockConnection();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.accept(conn);
    conn._recv(handshake("json"));
    return expect(conn.sent[0]).toEqual(duplex.handshake.accept);
  });

  it("handles registered function calls after accept", function() {
    const conn = new MockConnection();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("echo", testServices.echo);
    rpc.accept(conn);
    conn._recv(handshake("json"));
    const req = {
      type: duplex.request,
      method: "echo",
      id: 1,
      payload: {
        foo: "bar"
      }
    };
    conn._recv(JSON.stringify(req));
    expect(conn.sent.length).toEqual(2);
    return expect(JSON.parse(conn.sent[1])).toEqual({
      type: duplex.reply,
      id: 1,
      payload: {
        foo: "bar"
      }
    });
  });

  it("handles registered function calls after handshake", function() {
    const conn = new MockConnection();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("echo", testServices.echo);
    const peer = rpc.handshake(conn);
    conn._recv(duplex.handshake.accept);
    const req = {
      type: duplex.request,
      method: "echo",
      id: 1,
      payload: {
        foo: "bar"
      }
    };
    conn._recv(JSON.stringify(req));
    expect(conn.sent.length).toEqual(2);
    return expect(JSON.parse(conn.sent[1])).toEqual({
      type: duplex.reply,
      id: 1,
      payload: {
        foo: "bar"
      }
    });
  });

  it("calls remote peer functions after handshake", function() {
    const conn = new MockConnection();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    const peer = rpc.handshake(conn);
    conn._recv(duplex.handshake.accept);
    const args =
      {foo: "bar"};
    const reply =
      {baz: "qux"};
    let replied = false;
    runs(function() {
      peer.call("callAfterHandshake", args, function(err: object, rep: object) {
        expect(rep).toEqual(reply);
        return replied = true;
      });
      return conn._recv(JSON.stringify({
        type: duplex.reply,
        id: 1,
        payload: reply
      })
      );
    });
    return waitsFor(() => replied);
  });

  it("calls remote peer functions after accept", function() {
    const conn = new MockConnection();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    const peer = rpc.accept(conn);
    conn._recv(handshake("json"));
    const args =
      {foo: "bar"};
    const reply =
      {baz: "qux"};
    let replied = false;
    runs(function() {
      peer.call("callAfterAccept", args, function(err: object, rep: object) {
        expect(rep).toEqual(reply);
        return replied = true;
      });
      return conn._recv(JSON.stringify({
        type: duplex.reply,
        id: 1,
        payload: reply
      })
      );
    });
    return waitsFor(() => replied);
  });

  it("can do all handshake, accept, call, and handle", function() {
    const [conn1, conn2] = connectionPair();
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("echo-tag", (ch: Duplex.Channel) =>
      ch.onrecv = function(err: object, obj: object) {
        (<any>obj).tag = true;
        return ch.send(obj);
      }
    );
    let ready = 0;
    let [peer1, peer2]: Array<Duplex.Peer> = [null, null];
    runs(function() {
      peer1 = rpc.accept(conn1, () => ready++);
      return peer2 = rpc.handshake(conn2, () => ready++);
    });
    waitsFor(() => ready === 2);
    let replies = 0;
    runs(function() {
      peer1.call("echo-tag", {from: "peer1"}, function(err: object, rep: object) {
        expect(rep).toEqual({from: "peer1", tag: true});
        return replies++;
      });
      return peer2.call("echo-tag", {from: "peer2"}, function(err: object, rep: object) {
        expect(rep).toEqual({from: "peer2", tag: true});
        return replies++;
      });
    });
    return waitsFor(() => replies === 2);
  });

  it("streams multiple results", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("count", testServices.generator);
    let count = 0;
    return peerPair(rpc, (client: Duplex.Peer, _: Duplex.Peer) =>
      client.call("count", 5, function(err: object, rep: object) {
        count += (<any>rep).num;
        if (count === 15) {
          return done();
        }
      })
    );
  });

  it("streams multiple arguments", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("adder", testServices.adder);
    return peerPair(rpc, function(client: Duplex.Peer, _: Duplex.Peer) {
      const ch = client.open("adder");
      ch.onrecv = function(err: object, total: number) {
        expect(total).toEqual(15);
        return done();
      };
      return [1, 2, 3, 4, 5].map((num) =>
        ch.send(num, num !== 5));
    });
  });

  it("supports other codecs for serialization", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(b64json);
    rpc.register("echo", testServices.echo);
    return peerPair(rpc, (client: Duplex.Peer, server: Duplex.Peer) =>
      client.call("echo", {foo: "bar"}, function(err: object, rep: object) {
        expect(rep).toEqual({foo: "bar"});
        return done();
      })
    );
  });

  it("maintains optional ext from request to reply", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("echo", testServices.echo);
    return peerPair(rpc, function(client: Duplex.Peer, server: Duplex.Peer) {
      const ch = client.open("echo");
      ch.ext = {"hidden": "metadata"};
      ch.onrecv = function(err: object, reply: object) {
        expect(reply).toEqual({"foo": "bar"});
        expect(JSON.parse(server.conn.sent[1])["ext"])
          .toEqual({"hidden": "metadata"});
        return done();
      };
      return ch.send({"foo": "bar"}, false);
    });
  });

  it("registers func for traditional RPC methods and callbacks", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.registerFunc("callback", (args: Array<any>, reply: (cbReply: string) => any | void, ch: Duplex.Channel) =>
      ch.call(args[0], args[1], (err: object, cbReply: string) => reply(cbReply))
    );
    return peerPair(rpc, function(client: Duplex.Peer, server: Duplex.Peer) {
      const upper = rpc.callbackFunc((s: string, r: (s:string) => any) => r(s.toUpperCase()));
      return client.call("callback", [upper, "hello"], function(err: object, rep: string) {
        expect(rep).toEqual("HELLO");
        return done();
      });
    });
  });

  interface Error {
    code: number,
    message: string
  }

  it("lets handlers return error", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("error", testServices.error);
    return peerPair(rpc, (client: Duplex.Peer, server: Duplex.Peer) =>
      client.call("error", {foo: "bar"}, function(err: Error, rep: any) {
        expect(err["code"]).toEqual(testErrorCode);
        expect(err["message"]).toEqual(testErrorMessage);
        return done();
      })
    );
  });

  return it("lets handlers return error mid-stream", function(done: () => any) {
    const rpc: Duplex.RPC = new duplex.RPC(duplex.JSON);
    rpc.register("count", testServices.errorAfter2);
    let count = 0;
    return peerPair(rpc, (client: Duplex.Peer, server: Duplex.Peer) =>
      client.call("count", 5, function(err: Error, rep: string) {
        if (err != null) {
          expect(err["code"]).toEqual(testErrorCode);
          expect(err["message"]).toEqual(testErrorMessage);
          expect(count).toEqual(2);
          done();
        }
        return count += 1;
      })
    );
  });
});

function __range__(left: any, right: any, inclusive: any): Array<number> {
  let range = [];
  let ascending = left < right;
  let end = !inclusive ? right : ascending ? right + 1 : right - 1;
  for (let i = left; ascending ? i < end : i > end; ascending ? i++ : i--) {
    range.push(i);
  }
  return range;
}