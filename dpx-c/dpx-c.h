#include <msgpack.h>
#include <stdio.h>
#include <stdlib.h>
#include <task.h>
#include <unistd.h>

#include "uthash.h"

//#define NDEBUG
#include <assert.h>

// ------------------------------- { constants } ------------------------------
#define DPX_TASK_STACK_SIZE 65536

// -------------------------------- { errors } --------------------------------
#define DPX_ERROR_NONE 0
#define DPX_ERROR_FATAL -50
#define DPX_ERROR unsigned long

#define DPX_ERROR_FREEING 1

#define DPX_ERROR_CHAN_CLOSED 10
#define DPX_ERROR_CHAN_FRAME 11

#define DPX_ERROR_NETWORK_FAIL 20
#define DPX_ERROR_NETWORK_NOTALL 21

#define DPX_ERROR_PEER_ALREADYCLOSED 30

// ------------------------- { forward declarations } -------------------------

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
	Channel *outgoing
	DPX_ERROR err;
	char* method;
};

typedef struct _dpx_channel dpx_channel;

void dpx_channel_free(dpx_channel* c);
dpx_channel* dpx_channel_new_client(dpx_peer *p, char* method);
dpx_channel* dpx_channel_new_server(dpx_duplex_conn *conn, dpx_frame *frame);

void dpx_channel_close(dpx_channel *c, DPX_ERROR err);
DPX_ERROR dpx_channel_error(dpx_channel *c);
dpx_frame* dpx_channel_receive_frame(dpx_channel *c);
DPX_ERROR dpx_channel_send_frame(dpx_channel *c, dpx_frame *frame);
int dpx_channel_handle_incoming(dpx_channel *c, dpx_frame *frame);
void dpx_channel_pump_outgoing(dpx_channel *c);

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

typedef struct _dpx_frame dpx_frame;

void dpx_frame_free(dpx_frame *frame);
dpx_frame* dpx_frame_new(dpx_channel *ch);

dpx_frame* dpx_frame_msgpack_from(msgpack_object *obj);
msgpack_sbuffer* dpx_frame_msgpack_to(dpx_frame *frame);

// -------------------------- { duplex connection } ---------------------------
#define DPX_DUPLEX_CONN_CHUNK 8192
#define DPX_DUPLEX_CONN_BUFFER 65536

struct _dpx_channel_map {
	int key;
	dpx_channel* value;
	UT_hash_handle hh;
};

typedef struct _dpx_channel_map dpx_channel_map;

struct _dpx_duplex_conn {
	QLock *lock;
	dpx_peer *peer;
	int connfd;
	Channel* writeCh;
	dpx_channel_map *channels;
};

typedef struct _dpx_duplex_conn dpx_duplex_conn;

void dpx_duplex_conn_free(dpx_duplex_conn *c); // BEWARE, FREE DOES NOT CALL CLOSE!
void dpx_duplex_conn_close(dpx_duplex_conn *c);
dpx_duplex_conn* dpx_duplex_conn_new(dpx_peer *p, int fd);

void dpx_duplex_conn_read_frames(dpx_duplex_conn *c);
void dpx_duplex_conn_write_frames(dpx_duplex_conn *c);
DPX_ERROR dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame);
void dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch);
void dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch);

// --------------------------------- { peer } ---------------------------------
#define DPX_PEER_RETRYMS 1000
#define DPX_PEER_RETRYATTEMPTS 20

extern int dpx_peer_index;

struct _dpx_peer_listener {
	int fd;
	struct _dpx_peer_listener next;
};

typedef struct _dpx_peer_listener dpx_peer_listener;

struct _dpx_peer_connection {
	dpx_duplex_conn *conn;
	struct _dpx_peer_connection next;
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

typedef struct _dpx_peer dpx_peer;

void dpx_peer_free(dpx_peer *p);
dpx_peer* dpx_peer_new();

void dpx_peer_accept_connection(dpx_peer *p, int fd);
int dpx_peer_next_conn(dpx_peer *p, dpx_duplex_conn **conn);
void dpx_peer_route_open_frames(dpx_peer *p);

dpx_channel* dpx_peer_open(dpx_peer *p, char *method);
int dpx_peer_handle_open(dpx_peer *p, dpx_duplex_conn *conn, dpx_frame *frame);
dpx_channel* dpx_peer_accept(dpx_peer *p);
DPX_ERROR dpx_peer_close(dpx_peer *p);
DPX_ERROR dpx_peer_connect(dpx_peer *p, char* addr, int port);
DPX_ERROR dpx_peer_bind(dpx_peer *p, char* addr, int port);