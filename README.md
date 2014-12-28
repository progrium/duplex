# Duplex + Duplex RPC

Duplex is a simple application communications library built on top of the SSH protocol stack. It gives you secure pipes over TCP and between processes that tunnel bi-directional data streams or message frames. It also works efficiently in-process. 

Duplex RPC is a layer on top of Duplex that gives you streaming RPC using the message codecs of your choice. It was designed for modern distributed architectures *and* application plugin architectures. Make your microservices communicate efficiently, or give your applications powerful extensibility. 

## Implementations

#### PoC1

The first pass at Duplex was written in Go with the intention of a direct port to C using libtask. Porting to C is a major goal of the project so that the Duplex API can easily be exposed across languages. No ports, just a solid C implementation with language binding wrappers. 

This was a success, though somewhat buggy, it validated much of the API and high level architectural decisions. The low level wire protocol was simple and throwaway. Long term goal was to sit on top of libchan or directly on top of HTTP2.

#### PoC2

The second pass at Duplex came when revisiting the SSH protocol stack. It achieves much of the same multiplexing and security features of TLS/HTTP2, but with a well known auth model, solid implementations, and a protocol that almost seemed to be designed for Duplex. 

This is a work in progress, but is focusing on the Go implementation. The goal is to simplify the implementation and API to be easily understood, while the SSH stack does the heavy lifting. So far, this implementation is already more reliable than the homegrown stack of PoC1. 

#### Next

Once PoC2 is finished, the plan is again to port to C, using an existing SSH implementation as a foundation. Perhaps libssh. Again, the ultimate goal is to have a C library implementation. 

## Architecture

#### Peers and Channels

The main primitives in Duplex are Peers and Channels. Peers are like advanced sockets that can connect and be connected to. They most closely resemble ZeroMQ sockets. Behind the scenes they are both an SSH server and client up to the SSH Connection Layer. There's no terminals or shelling out here, we just use the lower level protocols that give us encrypted, bi-directional, multiplexed streams. These streams are exposed as Channels. 

Channels are much closer to a regular socket connection, but are tunneled through the secure connections between Peers. Unlike regular TCP connections, Channels come with some metadata, including their intended service and key-value headers. The Channel API has basic send/recv calls, but also calls for sending and receiving message frames. It also utilizes what SSH would use for stderr to send error frames. 

Frames are just length prefixed payloads of bytes. Frames can go in either direction. This is a solid foundation for any application protocol. What's more, you can send Channels over Channels. Just think about that.

#### RPC Layer

All that is really a foundation for a flexible, streaming RPC layer. Since different languages will have their own idiomatic way of exposing RPC clients and servers around Duplex, the RPC layer is not in the core library. All the RPC layer does is provide a natural interface to calling and exposing functions as RPC services. An RPC service is a function that can take some input (zero or more objects) and can produce some output (zero or more objects). 

Objects are typed structures marshalled by your codec of choice, whether it's msgpack or protobufs. These serialized objects are then sent as frames over Duplex Channels. When Channels are opened, they pass a codec header that allows the other end to know how it should serialize objects. You could even support multiple codecs at once. 

Like ZeroMQ sockets, Peers let you connect up topologies however you want. Both ends are clients and servers. You can also connect multiple Peers together. At the RPC layer, it will round-robin requests over multiple remote Peers.

#### Plugin Architectures

The heart of any plugin system is usually hooks or "delegate" APIs plugin authors implement. Duplex RPC is ideal for this since its goal is to be available in all languages and will always include both server and client. Since it's bi-directional, it also supports callbacks and other advanced RPC mechanisms. All this while not forcing data types or weird message patterns on you. On top of that, because it's made for TCP, you can allow networked/distributed plugins. 

## API

The API has gone through many changes but this is currently what it looks like in rough form, written as Go interfaces.

	type Peer interface {
		// Options
		SetOption(option int, value interface{}) error
		GetOption(option int) interface{} 

		// Connections
		Connect(endpoint string) error
		Disconnect(endpoint string) error
		Bind(endpoint string) error
		Unbind(endpoint string) error

		// Remote Peers
		Peers() []string
		Drop(peer string) error
		NextPeer() (string, error)

		// Channels
		Accept() (ChannelMeta, Channel)
		Open(peer, service string, headers []string) (Channel, error)

		// Cleanup
		Shutdown() error
	}

	type ChannelMeta interface {
		Service() string
		Headers() []string
		Trailers() []string
	}

	type Channel interface {
		// Send and receive
		Write(data []byte) (int, error)
		Read(data []byte) (int, error)
	
		// Frames
		WriteFrame(frame []byte) error
		ReadFrame() ([]byte, error)
	
		// Errors
		WriteError(frame []byte) error
		ReadError() ([]byte, error)

		// EOF and Close
		CloseWrite() error
		Close() error
		CloseWithTrailers(trailers []string) error
	
		// Channels of Channels
		Open(service string, headers []string) (Channel, error)
		Accept() (ChannelMeta, Channel)
	
		// Attach to real sockets for gateways/proxies
		Join(rwc io.ReadWriteCloser)

		// Reference to ChannelMeta
		Meta() ChannelMeta
	}

## License

BSD
