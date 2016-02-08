
assert = (description, condition=false) ->
  # We assert assumed state so we can more easily catch bugs.
  # Do not assert if we *know* the user can get to it.
  # HOWEVER in development there are asserts instead of exceptions...
  throw Error "Assertion: #{description}" if not condition

simplex =
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
    type: simplex.request
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
    type: simplex.reply
    id: id
    payload: payload
  if more == true
    msg.more = more
  if ext?
    msg.ext = ext
  msg

errorMsg = (id, code, message, data, ext)->
  msg =
    type: simplex.reply
    id: id
    error:
      code: code
      message: message
  if data?
    msg.error.data = data
  if ext?
    msg.ext = ext
  msg


class simplex.RPC
  constructor: (@codec)->
    @encode = @codec[1]
    @decode = @codec[2]
    @registered = {}

  register: (method, func) ->
    @registered[method] = func

  _handshake: ->
    p = simplex.protocol
    "#{p.name}/#{p.version};#{@codec[0]}"

  handshake: (conn, onready) ->
    peer = new simplex.Peer(this, conn, onready)
    conn.onrecv = (data) ->
      if data[0] == "+"
        conn.onrecv = peer.onrecv
        peer._ready(peer)
      else
        assert "Bad handshake: "+data
    conn.send @_handshake()
    peer

  accept: (conn, onready) ->
    peer = new simplex.Peer(this, conn, onready)
    conn.onrecv = (data) ->
      # TODO: check handshake
      conn.onrecv = peer.onrecv
      conn.send(simplex.handshake.accept)
      peer._ready(peer)
    peer

class simplex.Peer
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

  call: (method, args, onreply) ->
    ch = new simplex.Channel(this, simplex.request, method, @ext)
    if onreply?
      ch.id = ++@lastId
      ch.onrecv = onreply
      @repChan[ch.id] = ch
    ch.send(args)

  open: (method, onreply) ->
    ch = new simplex.Channel(this, simplex.request, method, @ext)
    ch.id = ++@lastId
    @repChan[ch.id] = ch
    if onreply?
      ch.onrecv = onreply
    ch

  onrecv: (frame) =>
    msg = @rpc.decode(frame)
    # TODO: catch decode error
    switch msg.type
      when simplex.request
        if @reqChan[msg.id]?
          ch = @reqChan[msg.id]
          if msg.more == false
            delete @reqChan[msg.id]
        else
          ch = new simplex.Channel(this, simplex.reply, msg.method)
          if msg.id != undefined
            ch.id = msg.id
            if msg.more == true
              @reqChan[ch.id] = ch
          assert "Method not registerd", @rpc.registered[msg.method]?
          @rpc.registered[msg.method](ch)
        if msg.ext?
          ch.ext = msg.ext
        ch.onrecv(msg.payload, msg.more)
      when simplex.reply
        if msg.error?
          @repChan[msg.id].onerr msg.error
          delete @repChan[msg.id]
        else
          @repChan[msg.id].onrecv msg.payload
          if msg.more == false
            delete @repChan[msg.id]
      else
        assert "Invalid message"

class simplex.Channel
  constructor: (@peer, @type, @method, @ext) ->
    assert "Channel expects Peer", @peer.constructor.name == "Peer"
    @id = null
    @onrecv = ->
    @onerr = ->

  call: (method, args, onreply) ->
    ch = @peer.open(method, onreply)
    ch.ext = @ext
    ch.send(args)

  close: ->
    @peer.close()

  open: (method, onreply) ->
    @peer.open(method, onreply)

  send: (payload, more=false) ->
    switch @type
      when simplex.request
        @peer.conn.send @peer.rpc.encode(
          requestMsg(payload, @method, @id, more, @ext))
      when simplex.reply
        @peer.conn.send @peer.rpc.encode(
          replyMsg(@id, payload, more, @ext))
      else
        assert "Bad channel type"

  senderr: (code, message, data) ->
    assert "Not reply channel", @type != simplex.reply
    @peer.conn.send @peer.rpc.encode(
      errorMsg(@id, code, message, data, @context))

if window?
  # For use in the browser
  window.simplex = simplex
else
  # For Node / testing
  exports.simplex = simplex
