export {}

namespace Duplex {

  export let version = "0.1.0";
  export let protocol = {
    name: "SIMPLEX",
    version: "1.0"
  };
  export let request = "req";
  export let reply = "rep";
  export let handshake = {
    accept: "+OK"
  };

  // Builtin codecs
  export let Json = [
    "json",
    JSON.stringify,
    JSON.parse
  ];

  // Builtin connection wrappers
  export let wrap = {
    "websocket"(ws: WebSocket): Connection {
      const conn: Connection = {
        send(msg: string) { return ws.send(msg); },
        close() { return ws.close(); }
      };
      ws.onmessage = (event: { data: string | Array<string> }) => conn.onrecv(event.data);
      return conn;
    },

    "nodejs-websocket"(ws: any): Connection {
      const conn: Connection = {
        send(msg: string) { return ws.send(msg); },
        close() { return ws.close(); }
      };
      ws.on("text", (msg: string) => conn.onrecv(msg));
      return conn;
    }
  };

  export class RPC {
    constructor(public codec: Array<any>) {
      this.encode = this.codec[1];
      this.decode = this.codec[2];
      this.registered = {};
    }

    encode: (obj: object) => string;
    decode: (str: string) => Message;
    registered: any;

    register(method: string, handler: object): object {
      return this.registered[method] = handler;
    }

    unregister(method: string): boolean {
      return delete this.registered[method];
    }

    registerFunc(method: string, func: (args: object, f1: object, ch: Channel) => object): object {
      return this.register(method, (ch: Channel) =>
        ch.onrecv = (err: object, args: object) => func(args, ( (reply: any, more: boolean) => { if (more == null) { more = false; } return ch.send(reply, more); }), ch)
      );
    }

    callbackFunc(func: (args: object, f1: object, ch: Channel) => object): string {
      const name = `_callback.${UUIDv4()}`;
      this.registerFunc(name, func);
      return name;
    }

    _handshake(): string {
      const p = Duplex.protocol;
      return `${p.name}/${p.version};${this.codec[0]}`;
    }

    handshake(conn: Connection, onready: (peer: Peer) => object | void): object {
      const peer = new Duplex.Peer(this, conn, onready);
      conn.onrecv = function(data: Array<string>) {
        if (data[0] === "+") {
          conn.onrecv = peer.onrecv;
          return peer._ready(peer);
        } else {
          return assert(`Bad handshake: ${data}`, false);
        }
      };
      conn.send(this._handshake());
      return peer;
    }

    accept(conn: Connection, onready: (peer: Peer) => object | void): object {
      const peer: Peer = new Duplex.Peer(this, conn, onready);
      conn.onrecv = function(data: any) {
        // TODO: check handshake
        conn.onrecv = peer.onrecv;
        conn.send(Duplex.handshake.accept);
        return peer._ready(peer);
      };
      return peer;
    }
  };

  export class Peer {
    constructor(public rpc: RPC, public conn: Connection, onready: (peer: Peer) => object | void) {
      this.onrecv = this.onrecv.bind(this);
      if (onready == null) { onready = function({}) {}; }
      this.onready = onready;
      assert("Peer expects an RPC", (<any>this.rpc.constructor).name === "RPC");
      assert("Peer expects a connection", (this.conn != null));
      this.lastId = 0;
      this.ext = null;
      this.reqChan = {};
      this.repChan = {};
    }

    onready: (peer: Peer) => object | void;
    lastId: number;
    ext: object;
    reqChan: any;
    repChan: any;

    _ready(peer: Peer): object | void {
      return this.onready(peer);
    }

    close(): object {
      return this.conn.close();
    }

    call(method?: string, args?: object | number, callback?: object): any {
      const ch: Channel = new Duplex.Channel(this, Duplex.request, method, this.ext);
      if (callback != null) {
        ch.id = ++this.lastId;
        ch.onrecv = callback;
        this.repChan[ch.id] = ch;
      }
      return ch.send(args);
    }

    open(method: string, callback: object): Channel {
      const ch: Channel = new Duplex.Channel(this, Duplex.request, method, this.ext);
      ch.id = ++this.lastId;
      this.repChan[ch.id] = ch;
      if (callback != null) {
        ch.onrecv = callback;
      }
      return ch;
    }

    onrecv(frame: string): any {
      let ch: Channel;
      if (frame === "") {
        // ignore empty frames
        return;
      }
      const msg: Message = this.rpc.decode(frame);
      // TODO: catch decode error
      switch (msg.type) {
        case Duplex.request:
          if (this.reqChan[msg.id] != null) {
            ch = this.reqChan[msg.id];
            if (msg.more === false) {
              delete this.reqChan[msg.id];
            }
          } else {
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
          } else {
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

  export class Channel {
    constructor(public peer: Peer, public type: string, public method: string, public ext?: object) {
      assert("Channel expects Peer", (<any>this.peer.constructor).name === "Peer");
      this.id = null;
      this.onrecv = function() {};
    }

    id: number;
    onrecv: any;

    call(method: string, args: string, callback: object): object {
      const ch: Channel = this.peer.open(method, callback);
      ch.ext = this.ext;
      return ch.send(args);
    }

    close(): object {
      return this.peer.close();
    }

    open(method: string, callback: object): object {
      const ch: Channel = this.peer.open(method, callback);
      ch.ext = this.ext;
      return ch;
    }

    send(payload: object | number | string, more?: boolean): any {
      if (more == null) { more = false; }
      switch (this.type) {
        case Duplex.request:
          return this.peer.conn.send(this.peer.rpc.encode(
            requestMsg(payload, this.method, this.id, more, this.ext))
          );
        case Duplex.reply:
          return this.peer.conn.send(this.peer.rpc.encode(
            replyMsg(this.id, payload, more, this.ext))
          );
        default:
          return assert("Bad channel type", false);
      }
    }

    senderr(code: number, message: string, data: object): object {
      assert("Not reply channel", this.type === Duplex.reply);
      return this.peer.conn.send(this.peer.rpc.encode(
        errorMsg(this.id, code, message, data, this.ext))
      );
    }
  };

  // high level wrapper for Duplex over websocket
  //  * call buffering when not connected
  //  * default retries (very basic right now)
  //  * manages setting up peer / handshake
  export class API {
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
      this.queued = [];
      this.rpc = new Duplex.RPC(Duplex.Json);
      var connect = (url: string): any => {
        this.ws = new WebSocket(url);
        this.ws.onopen = () => {
          return this.rpc.handshake(Duplex.wrap.websocket(this.ws), (p: any) => {
            this.peer = p;
            return this.queued.map((args: Array<any>) =>
              ((args: any) => this.call(...args || []))(args));
          });
        };
        return this.ws.onclose = () => {
          return setTimeout((() => connect(url)), 2000);
        };
      };
      connect(url);
    }

    queued: Array<any>;
    rpc: RPC;
    ws: WebSocket;
    peer: Peer;

    call(...args: Array<any>): number {
      console.log(args);
      if (this.peer != null) {
        return this.peer.call(...args || []);
      } else {
        return this.queued.push(args);
      }
    }
  };

};

interface Message {
  type: string,
  payload?: string | number | object,
  method?: string,
  id?: number,
  more?: boolean,
  ext?: object,
  error?: { code: number, message: string, data?: object }
};

interface Connection {
  onrecv?(data: string | Array<string>): any,
  send(msg: string): any,
  close(): any
};


const assert = function(description: string, condition: boolean): void {
  // We assert assumed state so we can more easily catch bugs.
  // Do not assert if we *know* the user can get to it.
  // HOWEVER in development there are asserts instead of exceptions...
  if (condition == null) { condition = false; }
  if (!condition) { throw Error(`Assertion: ${description}`); }
};

const requestMsg = function(payload: object | number | string, method: string, id: number, more: boolean, ext: object): object {
  const msg: Message = {
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

const replyMsg = function(id: number, payload: object | number | string, more: boolean, ext: object): object {
  const msg: Message = {
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

const errorMsg = function(id: number, code: number, message: string, data: object, ext: object): object {
  const msg: Message = {
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

// create duplex instance from namespace Duplex
// This design pattern was used to allow classes as properties with TypeScript, 
// while still being abel to export the Duplex object as originialy designed
let duplexInstance = {
  version: Duplex.version,
  protocol: Duplex.protocol,
  request: Duplex.request,
  reply: Duplex.reply,
  handshake: Duplex.handshake,
  JSON: Duplex.Json, // namespace property name changed because of colision with global 'JSON' object
  wrap: Duplex.wrap,
  RPC: Duplex.RPC,
  Peer: Duplex.Peer,
  Channel: Duplex.Channel,
  API: Duplex.API
};

if (typeof window !== 'undefined' && window !== null) {
  // For use in the browser
  (<any>window).duplex = duplexInstance;
} else {
  // For Node / testing
  (<any>exports).duplex = duplexInstance;
}

function __guard__(value: any, transform: any): void {
  return (typeof value !== 'undefined' && value !== null) ? transform(value) : undefined;
}