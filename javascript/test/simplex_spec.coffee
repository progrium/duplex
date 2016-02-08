{simplex} = require('../dist/simplex.js')
btoa = require('btoa')
atob = require('atob')

class MockConnection
  constructor: ->
    @sent = []
    @closed = false
    @onrecv = ->

  close: ->
    @closed = true

  send: (frame) ->
    #console.log(frame)
    @sent.push frame

  _recv: (frame) ->
    @onrecv frame

class PairedConnection
  constructor: ->
    @pairedWith = null
    @sent = []
    @closed = false
    @onrecv = ->

  close: ->
    @closed = true
    @pairedWith?.close()

  send: (frame) ->
    @sent.push frame
    @pairedWith.onrecv(frame)

connectionPair = ->
  conn1 = new PairedConnection()
  conn2 = new PairedConnection()
  conn1.pairedWith = conn2
  conn2.pairedWith = conn1
  [conn1, conn2]

peerPair = (rpc, onready) ->
  [conn1, conn2] = connectionPair()
  peer1 = rpc.accept conn1
  peer2 = rpc.handshake conn2, (peer2) ->
    onready(peer1, peer2)


handshake = (codec) ->
  p = simplex.protocol
  "#{p.name}/#{p.version};#{codec}"

testServices =
  echo: (ch) ->
    ch.onrecv = (obj) ->
      ch.send obj
  generator: (ch) ->
    ch.onrecv = (count) ->
      for num in [1..count]
        ch.send({num: num}, num != count)
  adder: (ch) ->
    total = 0
    ch.onrecv = (num, more) ->
      total += num
      if not more
        ch.send(total)


b64json = [
  "b64json"
  (obj) -> btoa(JSON.stringify(obj))
  (str) -> JSON.parse(atob(str))
]

describe "simplex module", ->
  it "has a version", ->
    expect(simplex.version).toBeDefined()

describe "simple RPC", ->
  it "handshakes", ->
    conn = new MockConnection()
    rpc = new simplex.RPC(simplex.JSON)
    rpc.handshake conn, ->
      expect(conn.sent[0]).toEqual handshake("json")

  it "accepts handshakes", ->
    conn = new MockConnection()
    rpc = new simplex.RPC(simplex.JSON)
    rpc.accept(conn)
    conn._recv handshake("json")
    expect(conn.sent[0]).toEqual simplex.handshake.accept

  it "handles registered function calls after accept", ->
    conn = new MockConnection()
    rpc = new simplex.RPC(simplex.JSON)
    rpc.register("echo", testServices.echo)
    rpc.accept(conn)
    conn._recv handshake("json")
    req =
      type: simplex.request
      method: "echo"
      id: 1
      payload:
        foo: "bar"
    conn._recv JSON.stringify(req)
    expect(conn.sent.length).toEqual 2
    expect(JSON.parse(conn.sent[1])).toEqual
      type: simplex.reply
      id: 1
      payload:
        foo: "bar"

  it "handles registered function calls after handshake", ->
    conn = new MockConnection()
    rpc = new simplex.RPC(simplex.JSON)
    rpc.register("echo", testServices.echo)
    peer = rpc.handshake(conn)
    conn._recv simplex.handshake.accept
    req =
      type: simplex.request
      method: "echo"
      id: 1
      payload:
        foo: "bar"
    conn._recv JSON.stringify(req)
    expect(conn.sent.length).toEqual 2
    expect(JSON.parse(conn.sent[1])).toEqual
      type: simplex.reply
      id: 1
      payload:
        foo: "bar"

  it "calls remote peer functions after handshake", ->
    conn = new MockConnection()
    rpc = new simplex.RPC(simplex.JSON)
    peer = rpc.handshake(conn)
    conn._recv simplex.handshake.accept
    args =
      foo: "bar"
    reply =
      baz: "qux"
    replied = false
    runs ->
      peer.call "callAfterHandshake", args, (rep) ->
        expect(rep).toEqual reply
        replied = true
      conn._recv JSON.stringify
        type: simplex.reply
        id: 1
        payload: reply
    waitsFor -> replied

  it "calls remote peer functions after accept", ->
    conn = new MockConnection()
    rpc = new simplex.RPC(simplex.JSON)
    peer = rpc.accept(conn)
    conn._recv handshake("json")
    args =
      foo: "bar"
    reply =
      baz: "qux"
    replied = false
    runs ->
      peer.call "callAfterAccept", args, (rep) ->
        expect(rep).toEqual reply
        replied = true
      conn._recv JSON.stringify
        type: simplex.reply
        id: 1
        payload: reply
    waitsFor -> replied

  it "can do all handshake, accept, call, and handle", ->
    [conn1, conn2] = connectionPair()
    rpc = new simplex.RPC(simplex.JSON)
    rpc.register("echo-tag", (ch) ->
      ch.onrecv = (obj) ->
        obj.tag = true
        ch.send obj)
    ready = 0
    [peer1, peer2] = [null, null]
    runs ->
      peer1 = rpc.accept conn1, ->
        ready++
      peer2 = rpc.handshake conn2, ->
        ready++
    waitsFor -> ready == 2
    replies = 0
    runs ->
      peer1.call "echo-tag", {from: "peer1"}, (rep) ->
        expect(rep).toEqual {from: "peer1", tag: true}
        replies++
      peer2.call "echo-tag", {from: "peer2"}, (rep) ->
        expect(rep).toEqual {from: "peer2", tag: true}
        replies++
    waitsFor -> replies == 2

  it "streams multiple results", (done) ->
    rpc = new simplex.RPC(simplex.JSON)
    rpc.register("count", testServices.generator)
    count = 0
    peerPair rpc, (client, _) ->
      client.call "count", 5, (rep) ->
        count += rep.num
        if count == 15
          done()

  it "streams multiple arguments", (done) ->
    rpc = new simplex.RPC(simplex.JSON)
    rpc.register("adder", testServices.adder)
    peerPair rpc, (client, _) ->
      ch = client.open "adder"
      ch.onrecv = (total) ->
        expect(total).toEqual 15
        done()
      for num in [1..5]
        ch.send num, num != 5

  it "supports other codecs for serialization", (done) ->
    rpc = new simplex.RPC(b64json)
    rpc.register("echo", testServices.echo)
    peerPair rpc, (client, server) ->
      client.call "echo", {foo: "bar"}, (rep) ->
        expect(rep).toEqual {foo: "bar"}
        done()

  it "maintains optional ext from request to reply", (done) ->
    rpc = new simplex.RPC(simplex.JSON)
    rpc.register("echo", testServices.echo)
    peerPair rpc, (client, server) ->
      ch = client.open "echo"
      ch.ext = {"hidden": "metadata"}
      ch.onrecv = (reply) ->
        expect(reply).toEqual {"foo": "bar"}
        expect(JSON.parse(server.conn.sent[1])["ext"])
          .toEqual {"hidden": "metadata"}
        done()
      ch.send {"foo": "bar"}, false
