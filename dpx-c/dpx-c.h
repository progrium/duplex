#include <msgpack.h>
#include <stdio.h>
#include <stdlib.h>
#include <task.h>

#include "uthash.h"

// ------------------------------- { constants } ------------------------------
#define DPX_TASK_STACK_SIZE 65536

// -------------------------------- { errors } --------------------------------
#define DPX_ERROR_NONE 0
#define DPX_ERROR_FATAL -50
#define DPX_ERROR unsigned long

#define DPX_ERROR_FREEING -10
#define DPX_ERROR_CHAN_CLOSED -20
#define DPX_ERROR_CHAN_FRAME -30

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
	FILE* conn;
	Channel* writeCh;
	dpx_channel_map *channels;
};

typedef struct _dpx_duplex_conn dpx_duplex_conn;