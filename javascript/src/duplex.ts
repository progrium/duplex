export {}

const assert = function(description: string, condition: boolean): void {
  // We assert assumed state so we can more easily catch bugs.
  // Do not assert if we *know* the user can get to it.
  // HOWEVER in development there are asserts instead of exceptions...
  if (condition == null) { condition = false; }
  if (!condition) { throw Error(`Assertion: ${description}`); }
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

  // Builtin codecs
  JSON: [
    "json",
    JSON.stringify,
    JSON.parse
  ],

  // Builtin connection wrappers
  wrap: {
    "websocket"(ws: WebSocket): object {
      const conn = {
        send(msg: string) { return (<any>ws).send(msg); },
        close() { return (<any>ws).close(); }
      };
      (<any>ws).onmessage = (event: object) => (<any>conn).onrecv((<any>event).data);
      return conn;
    },

    "nodejs-websocket"(ws: object) {
      const conn = {
        send(msg: string) { return (<any>ws).send(msg); },
        close() { return (<any>ws).close(); }
      };
      (<any>ws).on("text", (msg: string) => (<any>conn).onrecv(msg));
      return conn;
    }
  }
};

const requestMsg = function(payload: object | number | string, method: string, id: number, more: boolean, ext: object | undefined): object {
  const msg = {
    type: duplex.request,
    method,
    payload
  };
  if (id != null) {
    (<any>msg).id = id;
  }
  if (more === true) {
    (<any>msg).more = more;
  }
  if (ext != null) {
    (<any>msg).ext = ext;
  }
  return msg;
};

const replyMsg = function(id: number, payload: object | number | string, more: boolean, ext: object | undefined): object {
  const msg = {
    type: duplex.reply,
    id,
    payload
  };
  if (more === true) {
    (<any>msg).more = more;
  }
  if (ext != null) {
    (<any>msg).ext = ext;
  }
  return msg;
};

const errorMsg = function(id: number, code: number, message: string, data: object | undefined, ext: object | undefined): object {
  const msg = {
    type: duplex.reply,
    id,
    error: {
      code,
      message
    }
  };
  if (data != null) {
    (<any>msg.error).data = data;
  }
  if (ext != null) {
    (<any>msg).ext = ext;
  }
  return msg;
};

const UUIDv4 = function(): string {
  let d = new Date().getTime();
  if (typeof __guard__(typeof window !== 'undefined' && window !== null ? window.performance : undefined, (x: object) => (<any>x).now) === "function") {
    d += performance.now(); // use high-precision timer if available
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    let r = ((d + (Math.random()*16))%16) | 0;
    d = Math.floor(d/16);
    if (c !== 'x') {
      r = (r & 0x3) | 0x8;
    }
    return r.toString(16);
  });
};

(<any>duplex).RPC = class RPC {
  constructor(codec: object) {
    (<any>this).codec = codec;
    (<any>this).encode = (<any>this).codec[1];
    (<any>this).decode = (<any>this).codec[2];
    (<any>this).registered = {};
  }

  register(method: string, handler: object): object {
    return (<any>this).registered[method] = handler;
  }

  unregister(method: string): boolean {
    return delete (<any>this).registered[method];
  }

  registerFunc(method: string, func: (args: object, f1: object, ch: object) => object): object {
    return this.register(method, (ch: object) =>
      (<any>ch).onrecv = (err: object | null, args: object) => func(args, ( (reply: any, more: boolean | null) => { if (more == null) { more = false; } return (<any>ch).send(reply, more); }), ch)
    );
  }

  callbackFunc(func: (args: object, f1: object, ch: object) => object): string {
    const name = `_callback.${UUIDv4()}`;
    this.registerFunc(name, func);
    return name;
  }

  _handshake(): string {
    const p = duplex.protocol;
    return `${p.name}/${p.version};${(<any>this).codec[0]}`;
  }

  handshake(conn: object, onready: object | undefined): object {
    const peer = new (<any>duplex).Peer(this, conn, onready);
    (<any>conn).onrecv = function(data: Array<string>) {
      if (data[0] === "+") {
        (<any>conn).onrecv = peer.onrecv;
        return peer._ready(peer);
      } else {
        return assert(`Bad handshake: ${data}`, false);
      }
    };
    (<any>conn).send(this._handshake());
    return peer;
  }

  accept(conn: object, onready: object | undefined): object {
    const peer = new (<any>duplex).Peer(this, conn, onready);
    (<any>conn).onrecv = function(data: any) {
      // TODO: check handshake
      (<any>conn).onrecv = peer.onrecv;
      (<any>conn).send(duplex.handshake.accept);
      return peer._ready(peer);
    };
    return peer;
  }
};

(<any>duplex).Peer = class Peer {
  constructor(rpc: object, conn: object, onready: object | undefined) {
    this.onrecv = this.onrecv.bind(this);
    (<any>this).rpc = rpc;
    (<any>this).conn = conn;
    if (onready == null) { onready = function() {}; }
    (<any>this).onready = onready;
    assert("Peer expects an RPC", (<any>this).rpc.constructor.name === "RPC");
    assert("Peer expects a connection", ((<any>this).conn != null));
    (<any>this).lastId = 0;
    (<any>this).ext = null;
    (<any>this).reqChan = {};
    (<any>this).repChan = {};
  }

  _ready(peer: object): object {
    return (<any>this).onready(peer);
  }

  close(): object {
    return (<any>this).conn.close();
  }

  call(method: string, args: object | number, callback: object): object {
    const ch = new (<any>duplex).Channel(this, duplex.request, method, (<any>this).ext);
    if (callback != null) {
      ch.id = ++(<any>this).lastId;
      ch.onrecv = callback;
      (<any>this).repChan[ch.id] = ch;
    }
    return ch.send(args);
  }

  open(method: string, callback: object | undefined): object {
    const ch = new (<any>duplex).Channel(this, duplex.request, method, (<any>this).ext);
    ch.id = ++(<any>this).lastId;
    (<any>this).repChan[ch.id] = ch;
    if (callback != null) {
      ch.onrecv = callback;
    }
    return ch;
  }

  onrecv(frame: string): any {
    let ch;
    if (frame === "") {
      // ignore empty frames
      return;
    }
    const msg = (<any>this).rpc.decode(frame);
    // TODO: catch decode error
    switch (msg.type) {
      case duplex.request:
        if ((<any>this).reqChan[msg.id] != null) {
          ch = (<any>this).reqChan[msg.id];
          if (msg.more === false) {
            delete (<any>this).reqChan[msg.id];
          }
        } else {
          ch = new (<any>duplex).Channel(this, duplex.reply, msg.method);
          if (msg.id !== undefined) {
            ch.id = msg.id;
            if (msg.more === true) {
              (<any>this).reqChan[ch.id] = ch;
            }
          }
          assert("Method not registerd", ((<any>this).rpc.registered[msg.method] != null));
          (<any>this).rpc.registered[msg.method](ch);
        }
        if (msg.ext != null) {
          ch.ext = msg.ext;
        }
        return ch.onrecv(null, msg.payload, msg.more);
      case duplex.reply:
        if (msg.error != null) {
          if ((<any>this).repChan[msg.id] != null) {
            (<any>this).repChan[msg.id].onrecv(msg.error);
          }
          return delete (<any>this).repChan[msg.id];
        } else {
          if ((<any>this).repChan[msg.id] != null) {
            (<any>this).repChan[msg.id].onrecv(null, msg.payload, msg.more);
          }
          if (msg.more === false) {
            return delete (<any>this).repChan[msg.id];
          }
        }
        break;
      default:
        return assert("Invalid message", false);
    }
  }
};

(<any>duplex).Channel = class Channel {
  constructor(peer: object, type: string, method: string, ext: object | undefined) {
    (<any>this).peer = peer;
    (<any>this).type = type;
    (<any>this).method = method;
    (<any>this).ext = ext;
    assert("Channel expects Peer", (<any>this).peer.constructor.name === "Peer");
    (<any>this).id = null;
    (<any>this).onrecv = function() {};
  }

  call(method: string, args: string, callback: object | undefined): object {
    const ch = (<any>this).peer.open(method, callback);
    ch.ext = (<any>this).ext;
    return ch.send(args);
  }

  close(): object {
    return (<any>this).peer.close();
  }

  open(method: string, callback: object | undefined): object {
    const ch = (<any>this).peer.open(method, callback);
    ch.ext = (<any>this).ext;
    return ch;
  }

  send(payload: object | number | string, more: boolean | undefined): any {
    if (more == null) { more = false; }
    switch ((<any>this).type) {
      case duplex.request:
        return (<any>this).peer.conn.send((<any>this).peer.rpc.encode(
          requestMsg(payload, (<any>this).method, (<any>this).id, more, (<any>this).ext))
        );
      case duplex.reply:
        return (<any>this).peer.conn.send((<any>this).peer.rpc.encode(
          replyMsg((<any>this).id, payload, more, (<any>this).ext))
        );
      default:
        return assert("Bad channel type", false);
    }
  }

  senderr(code: number, message: string, data: object | undefined): object {
    assert("Not reply channel", (<any>this).type === duplex.reply);
    return (<any>this).peer.conn.send((<any>this).peer.rpc.encode(
      errorMsg((<any>this).id, code, message, data, (<any>this).ext))
    );
  }
};

// high level wrapper for duplex over websocket
//  * call buffering when not connected
//  * default retries (very basic right now)
//  * manages setting up peer / handshake
(<any>duplex).API = class API {
  constructor(endpoint: string) {
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
    (<any>this).queued = [];
    (<any>this).rpc = new (<any>duplex).RPC(duplex.JSON);
    var connect = (url: string) => {
      (<any>this).ws = new WebSocket(url);
      (<any>this).ws.onopen = () => {
        return (<any>this).rpc.handshake(duplex.wrap.websocket((<any>this).ws), (p: any) => {
          (<any>this).peer = p;
          return (<any>this).queued.map((args: Array<any>) =>
            ((args: any) => this.call(...args || []))(args));
        });
      };
      return (<any>this).ws.onclose = () => {
        return setTimeout((() => connect(url)), 2000);
      };
    };
    connect(url);
  }

  call(...args: Array<any>): Array<any> {
    console.log(args);
    if ((<any>this).peer != null) {
      return (<any>this).peer.call(...args || []);
    } else {
      return (<any>this).queued.push(args);
    }
  }
};

if (typeof window !== 'undefined' && window !== null) {
  // For use in the browser
  (<any>window).duplex = duplex;
} else {
  // For Node / testing
  (<any>exports).duplex = duplex;
}

function __guard__(value: any, transform: any): void {
  return (typeof value !== 'undefined' && value !== null) ? transform(value) : undefined;
}