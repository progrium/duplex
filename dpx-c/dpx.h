#include "uthash.h"

// ---------------------------- { threadsafe api } ----------------------------
// ----------- [and frankly, the only one you should be touching...] ----------

struct _dpx_context;
typedef struct _dpx_context dpx_context;

// BEFORE YOU DO ANYTHING WITH THIS API, YOU MUST INITIALISE THE COR-SCHEDULER.
dpx_context* dpx_init();
// OTHERWISE PREPARE FOR TROUBLE
void dpx_cleanup(dpx_context* c);

// -------------------------------- { errors } --------------------------------
#define DPX_ERROR_NONE 0
#define DPX_ERROR_FATAL -50
typedef unsigned long DPX_ERROR;

#define DPX_ERROR_FREEING 1

#define DPX_ERROR_CHAN_CLOSED 10
#define DPX_ERROR_CHAN_FRAME 11

#define DPX_ERROR_NETWORK_FAIL 20
#define DPX_ERROR_NETWORK_NOTALL 21

#define DPX_ERROR_PEER_ALREADYCLOSED 30

#define DPX_ERROR_DUPLEX_CLOSED 40

// ------------------------- { forward declarations } -------------------------

struct _dpx_duplex_conn;
typedef struct _dpx_duplex_conn dpx_duplex_conn;

struct _dpx_channel;
typedef struct _dpx_channel dpx_channel;

struct _dpx_frame;
typedef struct _dpx_frame dpx_frame;

struct _dpx_peer;
typedef struct _dpx_peer dpx_peer;

// --------------------------------- { peers } --------------------------------

// object tors
void dpx_peer_free(dpx_peer *p);
dpx_peer* dpx_peer_new(dpx_context *context);

// functions
dpx_channel* dpx_peer_open(dpx_peer *p, char *method); // method is copied
dpx_channel* dpx_peer_accept(dpx_peer *p);
DPX_ERROR dpx_peer_close(dpx_peer *p);
DPX_ERROR dpx_peer_connect(dpx_peer *p, char* addr, int port);
DPX_ERROR dpx_peer_bind(dpx_peer *p, char* addr, int port);

// FIXME below not cleaned up yet

// ------------------------------- { channels } -------------------------------

// object tors
void dpx_channel_free(dpx_channel *c);

// functions
void dpx_channel_close(dpx_channel *c, DPX_ERROR reason);
DPX_ERROR dpx_channel_error(dpx_channel *c);
dpx_frame* dpx_channel_receive_frame(dpx_channel *c);
DPX_ERROR dpx_channel_send_frame(dpx_channel *c, dpx_frame *frame);
int dpx_channel_handle_incoming(dpx_channel *c, dpx_frame *frame);

// properties
char* dpx_channel_method_get(dpx_channel *c); // returns method
char* dpx_channel_method_set(dpx_channel *c, char* method); // returns old one

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

	// below are meant to be modified.

	int type;
	int channel;

	char* method;
	dpx_header_map *headers; // use uthash.h to modify
	char* error;
	int last;

	char* payload;
	int payloadSize;
};

// object tors
void dpx_frame_free(dpx_frame *frame);
dpx_frame* dpx_frame_new(dpx_channel *ch);

// -------------------------- { duplex connection } ---------------------------

// void _dpx_duplex_conn_free(dpx_duplex_conn *c); // BEWARE, FREE DOES NOT CALL CLOSE!
// void _dpx_duplex_conn_close(dpx_duplex_conn *c);
// dpx_duplex_conn* _dpx_duplex_conn_new(dpx_peer *p, int fd);

// void _dpx_duplex_conn_read_frames(dpx_duplex_conn *c);
// void _dpx_duplex_conn_write_frames(dpx_duplex_conn *c);
// DPX_ERROR _dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame);
// void _dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch);
// void _dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch);