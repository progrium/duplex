"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const assert = function (description, condition) {
    if (condition == null) {
        condition = false;
    }
    if (!condition) {
        throw Error(`Assertion: ${description}`);
    }
};
let duplex = {
    version: "0.1.0",
    protocol: {
        name: "SIMPLEX",
        version: "1.0"
    },
    request: "req",
    reply: "rep",
    handshake: {
        accept: "+OK"
    },
    JSON: [
        "json",
        JSON.stringify,
        JSON.parse
    ],
    wrap: {
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
    }
};
const requestMsg = function (payload, method, id, more, ext) {
    const msg = {
        type: duplex.request,
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
        type: duplex.reply,
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
        type: duplex.reply,
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
duplex.RPC = class RPC {
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
        const p = duplex.protocol;
        return `${p.name}/${p.version};${this.codec[0]}`;
    }
    handshake(conn, onready) {
        const peer = new duplex.Peer(this, conn, onready);
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
        const peer = new duplex.Peer(this, conn, onready);
        conn.onrecv = function (data) {
            conn.onrecv = peer.onrecv;
            conn.send(duplex.handshake.accept);
            return peer._ready(peer);
        };
        return peer;
    }
};
duplex.Peer = class Peer {
    constructor(rpc, conn, onready) {
        this.onrecv = this.onrecv.bind(this);
        this.rpc = rpc;
        this.conn = conn;
        if (onready == null) {
            onready = function () { };
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
        const ch = new duplex.Channel(this, duplex.request, method, this.ext);
        if (callback != null) {
            ch.id = ++this.lastId;
            ch.onrecv = callback;
            this.repChan[ch.id] = ch;
        }
        return ch.send(args);
    }
    open(method, callback) {
        const ch = new duplex.Channel(this, duplex.request, method, this.ext);
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
            case duplex.request:
                if (this.reqChan[msg.id] != null) {
                    ch = this.reqChan[msg.id];
                    if (msg.more === false) {
                        delete this.reqChan[msg.id];
                    }
                }
                else {
                    ch = new duplex.Channel(this, duplex.reply, msg.method);
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
            case duplex.reply:
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
};
duplex.Channel = class Channel {
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
            case duplex.request:
                return this.peer.conn.send(this.peer.rpc.encode(requestMsg(payload, this.method, this.id, more, this.ext)));
            case duplex.reply:
                return this.peer.conn.send(this.peer.rpc.encode(replyMsg(this.id, payload, more, this.ext)));
            default:
                return assert("Bad channel type", false);
        }
    }
    senderr(code, message, data) {
        assert("Not reply channel", this.type === duplex.reply);
        return this.peer.conn.send(this.peer.rpc.encode(errorMsg(this.id, code, message, data, this.ext)));
    }
};
duplex.API = class API {
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
        this.rpc = new duplex.RPC(duplex.JSON);
        var connect = (url) => {
            this.ws = new WebSocket(url);
            this.ws.onopen = () => {
                return this.rpc.handshake(duplex.wrap.websocket(this.ws), (p) => {
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
        if (this.peer != null) {
            return this.peer.call(...args || []);
        }
        else {
            return this.queued.push(args);
        }
    }
};
if (typeof window !== 'undefined' && window !== null) {
    window.duplex = duplex;
}
else {
    exports.duplex = duplex;
}
function __guard__(value, transform) {
    return (typeof value !== 'undefined' && value !== null) ? transform(value) : undefined;
}
//# sourceMappingURL=duplex.js.map