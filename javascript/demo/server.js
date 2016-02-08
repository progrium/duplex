var ws = require("nodejs-websocket")
var simplex = require("../dist/simplex.js").simplex
var http = require('http');
var fs = require('fs');
var index = fs.readFileSync('index.html');
var simplexjs = fs.readFileSync('../dist/simplex.js');

// SERVE FILES
http.createServer(function (req, res) {
  if (req.url == "/simplex.js") {
    res.writeHead(200, {'Content-Type': 'text/javascript'});
    res.end(simplexjs);
  } else {
    res.writeHead(200, {'Content-Type': 'text/html'});
    res.end(index);
  }
}).listen(8000);
console.log("HTTP on 8000...")

// SETUP RPC
var rpc = new simplex.RPC(simplex.JSON)
rpc.register("echo", function(ch) {
  ch.onrecv = function(obj) {
    ch.send(obj)
  }
})
rpc.register("doMsgbox", function(ch) {
  ch.onrecv = function(text) {
    ch.call("msgbox", text)
  }
})

// WEBSOCKET SERVER
var server = ws.createServer(function (conn) {
  rpc.accept(simplex.wrap["nodejs-websocket"](conn))
}).listen(8001)
console.log("WS on 8001...")
