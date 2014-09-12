#include <msgpack.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#include "dpx.h"
#include "vendor/libtask/task.h"
#include "vendor/uuid/uuid.h"
#include "uthash.h"

//#define NDEBUG
#include <assert.h>

// ------------------------------- { constants } ------------------------------
#define DPX_TASK_STACK_SIZE 32768

#define UDP 0
#define TCP 1

#if defined(DEBUG) | defined(_DEBUG)
#ifndef DEBUG_FUNC
#define DEBUG_FUNC(...) do { __VA_ARGS__; } while(0)
#endif
#else
#ifndef DEBUG_FUNC
#define DEBUG_FUNC(...)
#endif
#endif

// -------------------------------- { errors } --------------------------------
// [ declarations have been moved to dpx.h in order for clients to utilise it ]

// -------------------------- { libtask thread com } --------------------------
struct _dpx_a {
	void* (*function)(void*);
	void *args;
};

typedef struct _dpx_a _dpx_a;

// Communicator with the libtask thread
void* _dpx_joinfunc(_dpx_a *a);

// ------------------------- { forward declarations } -------------------------

struct _dpx_duplex_conn;
typedef struct _dpx_duplex_conn dpx_duplex_conn;

struct _dpx_channel;
typedef struct _dpx_channel dpx_channel;

struct _dpx_frame;
typedef struct _dpx_frame dpx_frame;

struct _dpx_peer;
typedef struct _dpx_peer dpx_peer;

// --------------------------------- { peer } ---------------------------------
#define DPX_PEER_RETRYMS 1000
#define DPX_PEER_RETRYATTEMPTS 20

extern int _dpx_peer_index;

struct _dpx_peer_listener {
	int fd;
	struct _dpx_peer_listener *next;
};

typedef struct _dpx_peer_listener dpx_peer_listener;

struct _dpx_peer_connection {
	dpx_duplex_conn *conn;
	struct _dpx_peer_connection *next;
};

typedef struct _dpx_peer_connection dpx_peer_connection;

struct _dpx_peer {
	QLock *lock;

	uuid_t *uuid;

	dpx_peer_listener *listeners; // listener fds
	dpx_peer_connection *conns;
	al_channel* openFrames;
	al_channel* incomingChannels;
	int closed;
	int rrIndex;
	int chanIndex;
	int index;
	al_channel* firstConn;
};

void _dpx_peer_free(dpx_peer *p);
dpx_peer* _dpx_peer_new();

void _dpx_peer_accept_connection(dpx_peer *p, int fd, uuid_t *uuid);
int _dpx_peer_next_conn(dpx_peer *p, dpx_duplex_conn **conn);
void _dpx_peer_route_open_frames(dpx_peer *p);

dpx_channel* _dpx_peer_open(dpx_peer *p, char *method);
int _dpx_peer_handle_open(dpx_peer *p, dpx_duplex_conn *conn, dpx_frame *frame);
dpx_channel* _dpx_peer_accept(dpx_peer *p);
DPX_ERROR _dpx_peer_close(dpx_peer *p);
DPX_ERROR _dpx_peer_connect(dpx_peer *p, char* addr, int port);
DPX_ERROR _dpx_peer_bind(dpx_peer *p, char* addr, int port);

// dpx_peer_closed --> dpx.h
char* _dpx_peer_name(dpx_peer *p);

// ------------------------------- { channels } -------------------------------
#define DPX_CHANNEL_QUEUE_HWM 1024

struct _dpx_channel {
	QLock *lock;
	int id;
	dpx_peer *peer;
	al_channel *connCh;
	dpx_duplex_conn *conn;
	int server;
	int closed;
	int last;
	al_channel *incoming;
	al_channel *outgoing;
	al_channel *ocleanup;
	DPX_ERROR err;
	char* method;
};

void _dpx_channel_free(dpx_channel* c);
dpx_channel* _dpx_channel_new_client(dpx_peer *p, char* method);
dpx_channel* _dpx_channel_new_server(dpx_duplex_conn *conn, dpx_frame *frame);

void _dpx_channel_close(dpx_channel *c, DPX_ERROR err);
DPX_ERROR _dpx_channel_error(dpx_channel *c);
dpx_frame* _dpx_channel_receive_frame(dpx_channel *c);
DPX_ERROR _dpx_channel_send_frame(dpx_channel *c, dpx_frame *frame);
int _dpx_channel_handle_incoming(dpx_channel *c, dpx_frame *frame);
void _dpx_channel_pump_outgoing(dpx_channel *c);

// dpx_channel_method_get --> dpx.h
char* _dpx_channel_method_set(dpx_channel *c, char* method);
// dpx_channel_closed --> dpx.h
char* _dpx_channel_peer(dpx_channel *c);

// -------------------------------- { frames } --------------------------------
// #defines -> dpx.h

struct _dpx_header_map {
	char* key;
	char* value;
	UT_hash_handle hh; // hasher
};

typedef struct _dpx_header_map dpx_header_map;

// struct _dpx_frame -> dpx.h

// dpx_frame_free -> dpx.h
// dpx_frame_new -> dpx.h

dpx_frame* _dpx_frame_msgpack_from(msgpack_object *obj);
msgpack_sbuffer* _dpx_frame_msgpack_to(dpx_frame *frame);

// -------------------------- { duplex connection } ---------------------------
#define DPX_DUPLEX_CONN_CHUNK 8192
#define DPX_DUPLEX_CONN_BUFFER 65536
#define DPX_DUPLEX_CONN_FRAME_BUFFER 1024

struct dpx_channel_map {
	int key;
	dpx_channel* value;
	UT_hash_handle hh;
};

typedef struct dpx_channel_map dpx_channel_map;

struct _dpx_duplex_conn {
	QLock *lock;
	uuid_t *uuid;
	dpx_peer *peer;
	int connfd;
	al_channel* writeCh;
	dpx_channel_map *channels;
};

void _dpx_duplex_conn_free(dpx_duplex_conn *c); // BEWARE, FREE DOES NOT CALL CLOSE!
void _dpx_duplex_conn_close(dpx_duplex_conn *c);
dpx_duplex_conn* _dpx_duplex_conn_new(dpx_peer *p, int fd, uuid_t *uuid);

void _dpx_duplex_conn_read_frames(void *v);
void _dpx_duplex_conn_write_frames(dpx_duplex_conn *c);
DPX_ERROR _dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame);
void _dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch);
void _dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch);

// --------------------------- { alchan } -------------------------------------
#define ALCHAN_NONE -1
#define ALCHAN_CLOSED -2
#define ALCHAN_FULL -3

// Create a channel. The channel will be filled with elements of size
// 'elemsize' and will hold at most 'buffersize' elements before blocking.
// (0 means block immediately for processing.)
al_channel*	alchancreate(size_t elemsize, unsigned int buffersize);

// Close a channel. No more elements will be accepted.
void		alchanclose(al_channel *c);

// Free a channel. Fails if there are still elements within.
int			alchanfree(al_channel *c);

// Receive elements.
// Non-blocking.
int				alchannbrecv(al_channel *c, void *v);
void*			alchannbrecvp(al_channel *c);
unsigned long	alchannbrecvul(al_channel *c);

// Blocking. Waits for an element.
int				alchanrecv(al_channel *c, void *v);
void*			alchanrecvp(al_channel *c);
unsigned long	alchanrecvul(al_channel *c);

// Send elements.
// Non-blocking.
int				alchannbsend(al_channel *c, void *v);
int				alchannbsendp(al_channel *c, void *v);
int				alchannbsendul(al_channel *c, unsigned long v);

// Blocking. If the buffer is full, waits until a slot opens.
int				alchansend(al_channel *c, void *v);
int				alchansendp(al_channel *c, void *v);
int				alchansendul(al_channel *c, unsigned long v);

