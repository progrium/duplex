
assert = (description, condition=false) ->
  # We assert assumed state so we can more easily catch bugs.
  # Do not assert if we *know* the user can get to it.
  # HOWEVER in development there are asserts instead of exceptions...
  throw Error "Assertion: #{description}" if not condition

duplex =
  version: "0.1.0"
  protocol:
    name: "SIMPLEX"
    version: "1.0"
  request: "req"
  reply: "rep"
  handshake:
    accept: "+OK"

  # Builtin codecs
  JSON: [
    "json"
    JSON.stringify
    JSON.parse
  ]

  # Builtin connection wrappers
  wrap:
    "websocket": (ws) ->
      conn =
        send: (msg) -> ws.send(msg)
        close: -> ws.close()
      ws.onmessage = (event) -> conn.onrecv(event.data)
      conn

    "nodejs-websocket": (ws) ->
      conn =
        send: (msg) -> ws.send(msg)
        close: -> ws.close()
      ws.on "text", (msg) -> conn.onrecv(msg)
      conn

requestMsg = (payload, method, id, more, ext) ->
  msg =
    type: duplex.request
    method: method
    payload: payload
  if id?
    msg.id = id
  if more == true
    msg.more = more
  if ext?
    msg.ext = ext
  msg

replyMsg = (id, payload, more, ext) ->
  msg =
    type: duplex.reply
    id: id
    payload: payload
  if more == true
    msg.more = more
  if ext?
    msg.ext = ext
  msg

errorMsg = (id, code, message, data, ext)->
  msg =
    type: duplex.reply
    id: id
    error:
      code: code
      message: message
  if data?
    msg.error.data = data
  if ext?
    msg.ext = ext
  msg

UUIDv4 = ->
  d = new Date().getTime()
  if typeof window?.performance?.now == "function"
    d += performance.now() # use high-precision timer if available
  'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace /[xy]/g, (c) ->
    r = (d + Math.random()*16)%16 | 0
    d = Math.floor(d/16)
    if c != 'x'
      r = r & 0x3 | 0x8
    r.toString(16)

class duplex.RPC
  constructor: (@codec)->
    @encode = @codec[1]
    @decode = @codec[2]
    @registered = {}

  register: (method, handler) ->
    @registered[method] = handler

  unregister: (method) ->
    delete @registered[method]

  registerFunc: (method, func) ->
    @register method, (ch) ->
      ch.onrecv = (err, args) ->
        func(args, ((reply, more=false) -> ch.send reply, more), ch)

  callbackFunc: (func) ->
    name = "_callback."+UUIDv4()
    @registerFunc(name, func)
    name

  _handshake: ->
    p = duplex.protocol
    "#{p.name}/#{p.version};#{@codec[0]}"

  handshake: (conn, onready) ->
    peer = new duplex.Peer(this, conn, onready)
    conn.onrecv = (data) ->
      if data[0] == "+"
        conn.onrecv = peer.onrecv
        peer._ready(peer)
      else
        assert "Bad handshake: "+data
    conn.send @_handshake()
    peer

  accept: (conn, onready) ->
    peer = new duplex.Peer(this, conn, onready)
    conn.onrecv = (data) ->
      # TODO: check handshake
      conn.onrecv = peer.onrecv
      conn.send(duplex.handshake.accept)
      peer._ready(peer)
    peer

class duplex.Peer
  constructor: (@rpc, @conn, @onready=->) ->
    assert "Peer expects an RPC", @rpc.constructor.name == "RPC"
    assert "Peer expects a connection", @conn?
    @lastId = 0
    @ext = null
    @reqChan = {}
    @repChan = {}

  _ready: (peer) ->
    @onready(peer)

  close: ->
    @conn.close()

  call: (method, args, callback) ->
    ch = new duplex.Channel(this, duplex.request, method, @ext)
    if callback?
      ch.id = ++@lastId
      ch.onrecv = callback
      @repChan[ch.id] = ch
    ch.send(args)

  open: (method, callback) ->
    ch = new duplex.Channel(this, duplex.request, method, @ext)
    ch.id = ++@lastId
    @repChan[ch.id] = ch
    if callback?
      ch.onrecv = callback
    ch

  onrecv: (frame) =>
    if frame == ""
      # ignore empty frames
      return
    msg = @rpc.decode(frame)
    # TODO: catch decode error
    switch msg.type
      when duplex.request
        if @reqChan[msg.id]?
          ch = @reqChan[msg.id]
          if msg.more == false
            delete @reqChan[msg.id]
        else
          ch = new duplex.Channel(this, duplex.reply, msg.method)
          if msg.id != undefined
            ch.id = msg.id
            if msg.more == true
              @reqChan[ch.id] = ch
          assert "Method not registerd", @rpc.registered[msg.method]?
          @rpc.registered[msg.method](ch)
        if msg.ext?
          ch.ext = msg.ext
        ch.onrecv(null, msg.payload, msg.more)
      when duplex.reply
        if msg.error?
          @repChan[msg.id]?.onrecv msg.error
          delete @repChan[msg.id]
        else
          @repChan[msg.id]?.onrecv null, msg.payload, msg.more
          if msg.more == false
            delete @repChan[msg.id]
      else
        assert "Invalid message"

class duplex.Channel
  constructor: (@peer, @type, @method, @ext) ->
    assert "Channel expects Peer", @peer.constructor.name == "Peer"
    @id = null
    @onrecv = ->

  call: (method, args, callback) ->
    ch = @peer.open(method, callback)
    ch.ext = @ext
    ch.send(args)

  close: ->
    @peer.close()

  open: (method, callback) ->
    ch = @peer.open(method, callback)
    ch.ext = @ext
    ch

  send: (payload, more=false) ->
    switch @type
      when duplex.request
        @peer.conn.send @peer.rpc.encode(
          requestMsg(payload, @method, @id, more, @ext))
      when duplex.reply
        @peer.conn.send @peer.rpc.encode(
          replyMsg(@id, payload, more, @ext))
      else
        assert "Bad channel type"

  senderr: (code, message, data) ->
    assert "Not reply channel", @type == duplex.reply
    @peer.conn.send @peer.rpc.encode(
      errorMsg(@id, code, message, data, @ext))

if window?
  # For use in the browser
  window.duplex = duplex
else
  # For Node / testing
  exports.duplex = duplex
