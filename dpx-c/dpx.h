// ---------------------------- { threadsafe api } ----------------------------
// ----------- [and frankly, the only one you should be touching...] ----------

// BEFORE YOU DO ANYTHING WITH THIS API, YOU MUST INITIALISE THE COR-SCHEDULER.
void dpx_init();
// OTHERWISE PREPARE FOR TROUBLE
void dpx_cleanup();

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

void dpx_peer_free(dpx_peer *p);
dpx_peer* dpx_peer_new();

dpx_channel* dpx_peer_open(dpx_peer *p, char *method);
int dpx_peer_handle_open(dpx_peer *p, dpx_duplex_conn *conn, dpx_frame *frame);
dpx_channel* dpx_peer_accept(dpx_peer *p);
DPX_ERROR dpx_peer_close(dpx_peer *p);
DPX_ERROR dpx_peer_connect(dpx_peer *p, char* addr, int port);
DPX_ERROR dpx_peer_bind(dpx_peer *p, char* addr, int port);

// FIXME below not cleaned up yet

// ------------------------------- { channels } -------------------------------

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

void _dpx_frame_free(dpx_frame *frame);
dpx_frame* _dpx_frame_new(dpx_channel *ch);

dpx_frame* _dpx_frame_msgpack_from(msgpack_object *obj);
msgpack_sbuffer* _dpx_frame_msgpack_to(dpx_frame *frame);

// -------------------------- { duplex connection } ---------------------------

void _dpx_duplex_conn_free(dpx_duplex_conn *c); // BEWARE, FREE DOES NOT CALL CLOSE!
void _dpx_duplex_conn_close(dpx_duplex_conn *c);
dpx_duplex_conn* _dpx_duplex_conn_new(dpx_peer *p, int fd);

void _dpx_duplex_conn_read_frames(dpx_duplex_conn *c);
void _dpx_duplex_conn_write_frames(dpx_duplex_conn *c);
DPX_ERROR _dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame);
void _dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch);
void _dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch);