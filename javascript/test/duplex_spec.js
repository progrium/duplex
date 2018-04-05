"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const { duplex } = require('../dist/duplex.js');
const btoa = require('btoa');
const atob = require('atob');
const testErrorCode = 1000;
const testErrorMessage = "Test message";
class MockConnection {
    constructor() {
        this.sent = [];
        this.closed = false;
        this.pairedWith = null;
        this.onrecv = function () { };
    }
    close() {
        return this.closed = true;
    }
    send(frame) {
        this.sent.push(frame);
        if (this.pairedWith) {
            return this.pairedWith.onrecv(frame);
        }
    }
    _recv(frame) {
        return this.onrecv(frame);
    }
}
const connectionPair = function () {
    const conn1 = new MockConnection();
    const conn2 = new MockConnection();
    conn1.pairedWith = conn2;
    conn2.pairedWith = conn1;
    return [conn1, conn2];
};
const peerPair = function (rpc, onready) {
    let peer2;
    const [conn1, conn2] = connectionPair();
    const peer1 = rpc.accept(conn1);
    return peer2 = rpc.handshake(conn2, (peer2) => onready(peer1, peer2));
};
const handshake = function (codec) {
    const p = duplex.protocol;
    return `${p.name}/${p.version};${codec}`;
};
const testServices = {
    echo(ch) {
        return ch.onrecv = (err, obj) => ch.send(obj);
    },
    generator(ch) {
        return ch.onrecv = (err, count) => __range__(1, count, true).map((num) => ch.send({ num }, num !== count));
    },
    adder(ch) {
        let total = 0;
        return ch.onrecv = function (err, num, more) {
            total += num;
            if (!more) {
                return ch.send(total);
            }
        };
    },
    error(ch) {
        return ch.onrecv = () => ch.senderr(testErrorCode, testErrorMessage);
    },
    errorAfter2(ch) {
        return ch.onrecv = (err, count) => (() => {
            const result = [];
            for (let num = 1; num <= count; num++) {
                ch.send({ num }, num !== count);
                if (num === 2) {
                    ch.senderr(testErrorCode, testErrorMessage);
                    break;
                }
                else {
                    result.push(undefined);
                }
            }
            return result;
        })();
    }
};
const b64json = [
    "b64json",
    (obj) => btoa(JSON.stringify(obj)),
    (str) => JSON.parse(atob(str))
];
describe("duplex RPC", function () {
    it("handshakes", function () {
        const conn = new MockConnection();
        const rpc = new duplex.RPC(duplex.JSON);
        return rpc.handshake(conn, () => expect(conn.sent[0]).toEqual(handshake("json")));
    });
    it("accepts handshakes", function () {
        const conn = new MockConnection();
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.accept(conn);
        conn._recv(handshake("json"));
        return expect(conn.sent[0]).toEqual(duplex.handshake.accept);
    });
    it("handles registered function calls after accept", function () {
        const conn = new MockConnection();
        const rpc = new duplex.RPC(duplex.JSON);
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
    it("handles registered function calls after handshake", function () {
        const conn = new MockConnection();
        const rpc = new duplex.RPC(duplex.JSON);
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
    it("calls remote peer functions after handshake", function (done) {
        const conn = new MockConnection();
        const rpc = new duplex.RPC(duplex.JSON);
        const peer = rpc.handshake(conn);
        conn._recv(duplex.handshake.accept);
        const args = { foo: "bar" };
        const reply = { baz: "qux" };
        peer.call("callAfterHandshake", args, function (err, rep) {
            expect(rep).toEqual(reply);
            done();
        });
        conn._recv(JSON.stringify({
            type: duplex.reply,
            id: 1,
            payload: reply
        }));
    });
    it("calls remote peer functions after accept", function (done) {
        const conn = new MockConnection();
        const rpc = new duplex.RPC(duplex.JSON);
        const peer = rpc.accept(conn);
        conn._recv(handshake("json"));
        const args = { foo: "bar" };
        const reply = { baz: "qux" };
        peer.call("callAfterAccept", args, function (err, rep) {
            expect(rep).toEqual(reply);
            done();
        });
        conn._recv(JSON.stringify({
            type: duplex.reply,
            id: 1,
            payload: reply
        }));
    });
    it("can do all handshake, accept, call, and handle", function (done) {
        const [conn1, conn2] = connectionPair();
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.register("echo-tag", (ch) => ch.onrecv = function (err, obj) {
            obj.tag = true;
            return ch.send(obj);
        });
        let ready = 0;
        let [peer1, peer2] = [null, null];
        peer1 = rpc.accept(conn1, () => ready++);
        peer2 = rpc.handshake(conn2, () => ready++);
        let replies = 0;
        setTimeout(function () {
            peer1.call("echo-tag", { from: "peer1" }, function (err, rep) {
                expect(rep).toEqual({ from: "peer1", tag: true });
                replies++;
                if (replies === 2) {
                    done();
                }
            });
            peer2.call("echo-tag", { from: "peer2" }, function (err, rep) {
                expect(rep).toEqual({ from: "peer2", tag: true });
                replies++;
                if (replies === 2) {
                    done();
                }
            });
        }, 1);
    });
    it("streams multiple results", function (done) {
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.register("count", testServices.generator);
        let count = 0;
        return peerPair(rpc, (client, _) => client.call("count", 5, function (err, rep) {
            count += rep.num;
            if (count === 15) {
                return done();
            }
        }));
    });
    it("streams multiple arguments", function (done) {
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.register("adder", testServices.adder);
        return peerPair(rpc, function (client, _) {
            const ch = client.open("adder");
            ch.onrecv = function (err, total) {
                expect(total).toEqual(15);
                return done();
            };
            return [1, 2, 3, 4, 5].map((num) => ch.send(num, num !== 5));
        });
    });
    it("supports other codecs for serialization", function (done) {
        const rpc = new duplex.RPC(b64json);
        rpc.register("echo", testServices.echo);
        return peerPair(rpc, (client, server) => client.call("echo", { foo: "bar" }, function (err, rep) {
            expect(rep).toEqual({ foo: "bar" });
            return done();
        }));
    });
    it("maintains optional ext from request to reply", function (done) {
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.register("echo", testServices.echo);
        return peerPair(rpc, function (client, server) {
            const ch = client.open("echo");
            ch.ext = { "hidden": "metadata" };
            ch.onrecv = function (err, reply) {
                expect(reply).toEqual({ "foo": "bar" });
                expect(JSON.parse(server.conn.sent[1])["ext"])
                    .toEqual({ "hidden": "metadata" });
                return done();
            };
            return ch.send({ "foo": "bar" }, false);
        });
    });
    it("registers func for traditional RPC methods and callbacks", function (done) {
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.registerFunc("callback", (args, reply, ch) => ch.call(args[0], args[1], (err, cbReply) => reply(cbReply)));
        return peerPair(rpc, function (client, server) {
            const upper = rpc.callbackFunc((s, r) => r(s.toUpperCase()));
            return client.call("callback", [upper, "hello"], function (err, rep) {
                expect(rep).toEqual("HELLO");
                return done();
            });
        });
    });
    it("lets handlers return error", function (done) {
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.register("error", testServices.error);
        return peerPair(rpc, (client, server) => client.call("error", { foo: "bar" }, function (err, rep) {
            expect(err["code"]).toEqual(testErrorCode);
            expect(err["message"]).toEqual(testErrorMessage);
            return done();
        }));
    });
    return it("lets handlers return error mid-stream", function (done) {
        const rpc = new duplex.RPC(duplex.JSON);
        rpc.register("count", testServices.errorAfter2);
        let count = 0;
        return peerPair(rpc, (client, server) => client.call("count", 5, function (err, rep) {
            if (err != null) {
                expect(err["code"]).toEqual(testErrorCode);
                expect(err["message"]).toEqual(testErrorMessage);
                expect(count).toEqual(2);
                done();
            }
            return count += 1;
        }));
    });
});
function __range__(left, right, inclusive) {
    let range = [];
    let ascending = left < right;
    let end = !inclusive ? right : ascending ? right + 1 : right - 1;
    for (let i = left; ascending ? i < end : i > end; ascending ? i++ : i--) {
        range.push(i);
    }
    return range;
}
//# sourceMappingURL=duplex_spec.js.map