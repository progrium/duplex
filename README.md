# Megasock proof of concept

Megasock is a socket toolkit that makes building fast TCP "plumbing"
(proxies, gateways, routers, and more) very easy. This is a proof of concept
where the functionality is implemented in Python, however the idea is to
re-implement it in C and expose an API that makes it easy to provide
bindings in many languages. 

## Core idea: Socket joining

Almost every bit of TCP plumbing (that is, anything in between an
application client and application server) involves this pattern
represented in pseudo-code:

  Thread:
    while True:
      SocketA.send(SocketB.recv())
  Thread:
    while True:
      SocketB.send(SocketA.recv())

This is how you write proxies and, well, pretty much everything else is
a variation on a proxy (routers, balancers, gateways, tunnels, etc). What you do
around this pattern is what's interesting and is specific to your bit of
plumbing, whether it's a health-based load balancer, or a secure tunnel,
or a port forwarder. The idea is this pattern should be optimized and
made available to all languages. This is socket joining, the primary
primitive of Megasock:

  MegaSock_Join(SocketA, SocketB)

Everything is taken care of for you. You can unjoin them at any time.
You can *join* them at any time. In fact, that's a feature so you can
setup your socket/connection however you like, handle handshakes or
whatever, then join the streams. No matter what environment you use
Megasock, you get this simple API and everything is taken care of for
you, no matter if you're using an event loop, multithreading, greenlets. 

What's more, we optimize it on two levels: 1) the streams are joined by
a very tight C loop, 2) we use splice or sendfile to join the sockets
with "zero-copy", meaning we avoid moving data across the kernel/app
boundary. Now you can write custom proxies as fast as HAProxy in
the language of your choice. 

## Building on this primitive

Looking at our examples, you can see some interesting patterns and
applications you can build with socket joining. 

Write more...

## Another core primitive

inspect

## Messaging layer!

A whole new level
