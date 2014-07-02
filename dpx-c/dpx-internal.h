#include <msgpack.h>
#include <stdio.h>
#include <stdlib.h>
#include <task.h>
#include <unistd.h>

#include "dpx.h"
#include "uthash.h"

//#define NDEBUG
#include <assert.h>

// ------------------------------- { constants } ------------------------------
#define DPX_TASK_STACK_SIZE 65536

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
	dpx_peer_listener *listeners; // listener fds
	dpx_peer_connection *conns;
	Channel* openFrames;
	Channel* incomingChannels;
	int closed;
	int rrIndex;
	int chanIndex;
	int index;
	Channel* firstConn;
};

// dpx_peer_free -> dpx.h
// dpx_peer_new -> dpx.h

void _dpx_peer_accept_connection(dpx_peer *p, int fd);
int _dpx_peer_next_conn(dpx_peer *p, dpx_duplex_conn **conn);
void _dpx_peer_route_open_frames(dpx_peer *p);

dpx_channel* _dpx_peer_open(dpx_peer *p, char *method);
int _dpx_peer_handle_open(dpx_peer *p, dpx_duplex_conn *conn, dpx_frame *frame);
dpx_channel* _dpx_peer_accept(dpx_peer *p);
DPX_ERROR _dpx_peer_close(dpx_peer *p);
DPX_ERROR _dpx_peer_connect(dpx_peer *p, char* addr, int port);
DPX_ERROR _dpx_peer_bind(dpx_peer *p, char* addr, int port);

// ------------------------------- { channels } -------------------------------
#define DPX_CHANNEL_QUEUE_HWM 1024

struct _dpx_channel {
	QLock *lock;
	int id;
	dpx_peer *peer;
	Channel *connCh;
	dpx_duplex_conn *conn;
	int server;
	int closed;
	int last;
	Channel *incoming;
	Channel *outgoing;
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

// -------------------------------- { frames } --------------------------------
#define DPX_FRAME_OPEN 0
#define DPX_FRAME_DATA 1
#define DPX_FRAME_NOCH -1

#define DPX_PACK_ARRAY_SIZE 7

struct _dpx_header_map {
	char* key;
	char* value;
	UT_hash_handle hh; // hasher
};

typedef struct _dpx_header_map dpx_header_map;

struct _dpx_frame {
	Channel *errCh;
	dpx_channel *chanRef;

	int type;
	int channel;

	char* method;
	dpx_header_map *headers; // MUST ALWAYS INITIALISE TO NULL
	char* error;
	int last;

	char* payload;
	int payloadSize;
};

void _dpx_frame_free(dpx_frame *frame);
dpx_frame* _dpx_frame_new(dpx_channel *ch);

dpx_frame* _dpx_frame_msgpack_from(msgpack_object *obj);
msgpack_sbuffer* _dpx_frame_msgpack_to(dpx_frame *frame);

// -------------------------- { duplex connection } ---------------------------
#define DPX_DUPLEX_CONN_CHUNK 8192
#define DPX_DUPLEX_CONN_BUFFER 65536

struct dpx_channel_map {
	int key;
	dpx_channel* value;
	UT_hash_handle hh;
};

typedef struct dpx_channel_map dpx_channel_map;

struct _dpx_duplex_conn {
	QLock *lock;
	dpx_peer *peer;
	int connfd;
	Channel* writeCh;
	dpx_channel_map *channels;
};

void _dpx_duplex_conn_free(dpx_duplex_conn *c); // BEWARE, FREE DOES NOT CALL CLOSE!
void _dpx_duplex_conn_close(dpx_duplex_conn *c);
dpx_duplex_conn* _dpx_duplex_conn_new(dpx_peer *p, int fd);

void _dpx_duplex_conn_read_frames(dpx_duplex_conn *c);
void _dpx_duplex_conn_write_frames(dpx_duplex_conn *c);
DPX_ERROR _dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame);
void _dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch);
void _dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch);