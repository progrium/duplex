"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
var Duplex;
(function (Duplex) {
    Duplex.version = "0.1.0";
    Duplex.protocol = {
        name: "SIMPLEX",
        version: "1.0"
    };
    Duplex.request = "req";
    Duplex.reply = "rep";
    Duplex.handshake = {
        accept: "+OK"
    };
    Duplex.Json = [
        "json",
        JSON.stringify,
        JSON.parse
    ];
    Duplex.wrap = {
        "websocket"(ws) {
            const conn = {
                send(msg) { return ws.send(msg); },
                close() { return ws.close(); }
            };
            ws.onmessage = (event) => conn.onrecv(event.data);
            return conn;
        },
        "nodejs-websocket"(ws) {
            const conn = {
                send(msg) { return ws.send(msg); },
                close() { return ws.close(); }
            };
            ws.on("text", (msg) => conn.onrecv(msg));
            return conn;
        }
    };
    class RPC {
        constructor(codec) {
            this.codec = codec;
            this.encode = this.codec[1];
            this.decode = this.codec[2];
            this.registered = {};
        }
        register(method, handler) {
            return this.registered[method] = handler;
        }
        unregister(method) {
            return delete this.registered[method];
        }
        registerFunc(method, func) {
            return this.register(method, (ch) => ch.onrecv = (err, args) => func(args, ((reply, more) => { if (more == null) {
                more = false;
            } return ch.send(reply, more); }), ch));
        }
        callbackFunc(func) {
            const name = `_callback.${UUIDv4()}`;
            this.registerFunc(name, func);
            return name;
        }
        _handshake() {
            const p = Duplex.protocol;
            return `${p.name}/${p.version};${this.codec[0]}`;
        }
        handshake(conn, onready) {
            const peer = new Duplex.Peer(this, conn, onready);
            conn.onrecv = function (data) {
                if (data[0] === "+") {
                    conn.onrecv = peer.onrecv;
                    return peer._ready(peer);
                }
                else {
                    return assert(`Bad handshake: ${data}`, false);
                }
            };
            conn.send(this._handshake());
            return peer;
        }
        accept(conn, onready) {
            const peer = new Duplex.Peer(this, conn, onready);
            conn.onrecv = function (data) {
                conn.onrecv = peer.onrecv;
                conn.send(Duplex.handshake.accept);
                return peer._ready(peer);
            };
            return peer;
        }
    }
    Duplex.RPC = RPC;
    ;
    class Peer {
        constructor(rpc, conn, onready) {
            this.rpc = rpc;
            this.conn = conn;
            this.onrecv = this.onrecv.bind(this);
            if (onready == null) {
                onready = function ({}) { };
            }
            this.onready = onready;
            assert("Peer expects an RPC", this.rpc.constructor.name === "RPC");
            assert("Peer expects a connection", (this.conn != null));
            this.lastId = 0;
            this.ext = null;
            this.reqChan = {};
            this.repChan = {};
        }
        _ready(peer) {
            return this.onready(peer);
        }
        close() {
            return this.conn.close();
        }
        call(method, args, callback) {
            const ch = new Duplex.Channel(this, Duplex.request, method, this.ext);
            if (callback != null) {
                ch.id = ++this.lastId;
                ch.onrecv = callback;
                this.repChan[ch.id] = ch;
            }
            return ch.send(args);
        }
        open(method, callback) {
            const ch = new Duplex.Channel(this, Duplex.request, method, this.ext);
            ch.id = ++this.lastId;
            this.repChan[ch.id] = ch;
            if (callback != null) {
                ch.onrecv = callback;
            }
            return ch;
        }
        onrecv(frame) {
            let ch;
            if (frame === "") {
                return;
            }
            const msg = this.rpc.decode(frame);
            switch (msg.type) {
                case Duplex.request:
                    if (this.reqChan[msg.id] != null) {
                        ch = this.reqChan[msg.id];
                        if (msg.more === false) {
                            delete this.reqChan[msg.id];
                        }
                    }
                    else {
                        ch = new Duplex.Channel(this, Duplex.reply, msg.method);
                        if (msg.id !== undefined) {
                            ch.id = msg.id;
                            if (msg.more === true) {
                                this.reqChan[ch.id] = ch;
                            }
                        }
                        assert("Method not registerd", (this.rpc.registered[msg.method] != null));
                        this.rpc.registered[msg.method](ch);
                    }
                    if (msg.ext != null) {
                        ch.ext = msg.ext;
                    }
                    return ch.onrecv(null, msg.payload, msg.more);
                case Duplex.reply:
                    if (msg.error != null) {
                        if (this.repChan[msg.id] != null) {
                            this.repChan[msg.id].onrecv(msg.error);
                        }
                        return delete this.repChan[msg.id];
                    }
                    else {
                        if (this.repChan[msg.id] != null) {
                            this.repChan[msg.id].onrecv(null, msg.payload, msg.more);
                        }
                        if (msg.more === false) {
                            return delete this.repChan[msg.id];
                        }
                    }
                    break;
                default:
                    return assert("Invalid message", false);
            }
        }
    }
    Duplex.Peer = Peer;
    ;
    class Channel {
        constructor(peer, type, method, ext) {
            this.peer = peer;
            this.type = type;
            this.method = method;
            this.ext = ext;
            assert("Channel expects Peer", this.peer.constructor.name === "Peer");
            this.id = null;
            this.onrecv = function () { };
        }
        call(method, args, callback) {
            const ch = this.peer.open(method, callback);
            ch.ext = this.ext;
            return ch.send(args);
        }
        close() {
            return this.peer.close();
        }
        open(method, callback) {
            const ch = this.peer.open(method, callback);
            ch.ext = this.ext;
            return ch;
        }
        send(payload, more) {
            if (more == null) {
                more = false;
            }
            switch (this.type) {
                case Duplex.request:
                    return this.peer.conn.send(this.peer.rpc.encode(requestMsg(payload, this.method, this.id, more, this.ext)));
                case Duplex.reply:
                    return this.peer.conn.send(this.peer.rpc.encode(replyMsg(this.id, payload, more, this.ext)));
                default:
                    return assert("Bad channel type", false);
            }
        }
        senderr(code, message, data) {
            assert("Not reply channel", this.type === Duplex.reply);
            return this.peer.conn.send(this.peer.rpc.encode(errorMsg(this.id, code, message, data, this.ext)));
        }
    }
    Duplex.Channel = Channel;
    ;
    class API {
        constructor(endpoint) {
            const parts = endpoint.split(":");
            assert("Invalid endpoint", parts.length > 1);
            const scheme = parts.shift();
            let [protocol, codec] = scheme.split("+", 2);
            if (codec == null) {
                codec = "json";
            }
            assert("JSON is only supported codec", codec === "json");
            parts.unshift(protocol);
            const url = parts.join(":");
            this.queued = [];
            this.rpc = new Duplex.RPC(Duplex.Json);
            var connect = (url) => {
                this.ws = new WebSocket(url);
                this.ws.onopen = () => {
                    return this.rpc.handshake(Duplex.wrap.websocket(this.ws), (p) => {
                        this.peer = p;
                        return this.queued.map((args) => ((args) => this.call(...args || []))(args));
                    });
                };
                return this.ws.onclose = () => {
                    return setTimeout((() => connect(url)), 2000);
                };
            };
            connect(url);
        }
        call(...args) {
            console.log(args);
            if (this.peer != null) {
                return this.peer.call(...args || []);
            }
            else {
                return this.queued.push(args);
            }
        }
    }
    Duplex.API = API;
    ;
})(Duplex || (Duplex = {}));
;
;
;
const assert = function (description, condition) {
    if (condition == null) {
        condition = false;
    }
    if (!condition) {
        throw Error(`Assertion: ${description}`);
    }
};
const requestMsg = function (payload, method, id, more, ext) {
    const msg = {
        type: Duplex.request,
        method,
        payload
    };
    if (id != null) {
        msg.id = id;
    }
    if (more === true) {
        msg.more = more;
    }
    if (ext != null) {
        msg.ext = ext;
    }
    return msg;
};
const replyMsg = function (id, payload, more, ext) {
    const msg = {
        type: Duplex.reply,
        id,
        payload
    };
    if (more === true) {
        msg.more = more;
    }
    if (ext != null) {
        msg.ext = ext;
    }
    return msg;
};
const errorMsg = function (id, code, message, data, ext) {
    const msg = {
        type: Duplex.reply,
        id,
        error: {
            code,
            message
        }
    };
    if (data != null) {
        msg.error.data = data;
    }
    if (ext != null) {
        msg.ext = ext;
    }
    return msg;
};
const UUIDv4 = function () {
    let d = new Date().getTime();
    if (typeof __guard__(typeof window !== 'undefined' && window !== null ? window.performance : undefined, (x) => x.now) === "function") {
        d += performance.now();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
        let r = ((d + (Math.random() * 16)) % 16) | 0;
        d = Math.floor(d / 16);
        if (c !== 'x') {
            r = (r & 0x3) | 0x8;
        }
        return r.toString(16);
    });
};
let duplexInstance = {
    version: Duplex.version,
    protocol: Duplex.protocol,
    request: Duplex.request,
    reply: Duplex.reply,
    handshake: Duplex.handshake,
    JSON: Duplex.Json,
    wrap: Duplex.wrap,
    RPC: Duplex.RPC,
    Peer: Duplex.Peer,
    Channel: Duplex.Channel,
    API: Duplex.API
};
if (typeof window !== 'undefined' && window !== null) {
    window.duplex = duplexInstance;
}
else {
    exports.duplex = duplexInstance;
}
function __guard__(value, transform) {
    return (typeof value !== 'undefined' && value !== null) ? transform(value) : undefined;
}
//# sourceMappingURL=duplex.js.map