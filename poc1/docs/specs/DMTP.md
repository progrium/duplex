# DMTP

DMTP is a symmetric message framing protocol, inspired by ZeroMQ's ZMTP. 
However, right now it's not that interesting -- it's just msgpack objects.

Potentially it could be replaced with something else. Perhaps the HTTP/2
framing layer?

Otherwise, it's undocumented as the current implementation is evolving quite
a bit during development.