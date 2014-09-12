#include "dpx-internal.h"
#include <string.h> // for strcmp

void* _dpx_channel_free_helper(void* v) {
	_dpx_channel_free((dpx_channel*) v);
	return NULL;
}

void dpx_channel_free(dpx_channel *c) {
	_dpx_a a;
	a.function = &_dpx_channel_free_helper;
	a.args = c;

	_dpx_joinfunc(&a);
}

struct _dpx_channel_close_hs {
	dpx_channel *c;
	DPX_ERROR reason;
};

void* _dpx_channel_close_helper(void* v) {
	struct _dpx_channel_close_hs *h = (struct _dpx_channel_close_hs*) v;
	_dpx_channel_close(h->c, h->reason);
	return NULL;
}

void dpx_channel_close(dpx_channel *c, DPX_ERROR reason) {
	_dpx_a a;
	a.function = &_dpx_channel_close_helper;

	struct _dpx_channel_close_hs h;
	h.c = c;
	h.reason = reason;

	a.args = &h;

	_dpx_joinfunc(&a);
}

struct _dpx_channel_error_hs {
	dpx_channel *c;
	DPX_ERROR err;
};

void* _dpx_channel_error_helper(void* v) {
	struct _dpx_channel_error_hs *h = (struct _dpx_channel_error_hs*) v;
	h->err = _dpx_channel_error(h->c);
	return NULL;
}

DPX_ERROR dpx_channel_error(dpx_channel *c) {
	_dpx_a a;
	a.function = &_dpx_channel_error_helper;

	struct _dpx_channel_error_hs h;
	h.c = c;

	a.args = &h;

	_dpx_joinfunc(&a);
	return h.err;
}

void* _dpx_channel_receive_frame_helper(void* v) {
	return _dpx_channel_receive_frame((dpx_channel*)v);
}

dpx_frame* dpx_channel_receive_frame(dpx_channel *c) {
	_dpx_a a;
	a.function = &_dpx_channel_receive_frame_helper;
	a.args = c;

	return (dpx_frame*) _dpx_joinfunc(&a);
}

struct _dpx_channel_send_frame_hs {
	dpx_channel *c;
	dpx_frame *frame;
	DPX_ERROR err;
};

void* _dpx_channel_send_frame_helper(void* v) {
	struct _dpx_channel_send_frame_hs *h = (struct _dpx_channel_send_frame_hs*) v;
	h->err = _dpx_channel_send_frame(h->c, h->frame);
	return NULL;
}

DPX_ERROR dpx_channel_send_frame(dpx_channel *c, dpx_frame *frame) {
	_dpx_a a;
	a.function = &_dpx_channel_send_frame_helper;

	struct _dpx_channel_send_frame_hs h;
	h.c = c;
	h.frame = frame;

	a.args = &h;

	_dpx_joinfunc(&a);
	return h.err;
}

char* dpx_channel_method_get(dpx_channel *c) {
	return c->method;
}

struct _dpx_channel_method_set_hs {
	dpx_channel *c;
	char* method;
};

void* _dpx_channel_method_set_helper(void* v) {
	struct _dpx_channel_method_set_hs *h = (struct _dpx_channel_method_set_hs*) v;
	return _dpx_channel_method_set(h->c, h->method);
}

char* dpx_channel_method_set(dpx_channel *c, char* method) {
	_dpx_a a;
	a.function = &_dpx_channel_method_set_helper;

	struct _dpx_channel_method_set_hs h;
	h.c = c;
	h.method = method;

	a.args = &h;

	void* ret = _dpx_joinfunc(&a);

	return (char*)ret;
}

int dpx_channel_closed(dpx_channel *c) {
	return c->closed;
}

void* _dpx_channel_peer_helper(void* v) {
	dpx_channel *c = (dpx_channel*) v;
	return _dpx_channel_peer(c);
}

char* dpx_channel_peer(dpx_channel *c) {
	_dpx_a a;
	a.function = &_dpx_channel_peer_helper;
	a.args = c;

	void* ret = _dpx_joinfunc(&a);

	return (char*)ret;
}

// ----------------------------------------------------------------------------

dpx_channel* _dpx_channel_new() {
	dpx_channel* ptr = (dpx_channel*) malloc(sizeof(dpx_channel));

	ptr->lock = calloc(1, sizeof(QLock));
	ptr->id = 0;

	ptr->peer = NULL;
	ptr->connCh = alchancreate(sizeof(dpx_duplex_conn*), 0);
	ptr->conn = NULL;

	ptr->server = 0;
	ptr->closed = 0;
	ptr->last = 0;

	ptr->incoming = alchancreate(sizeof(dpx_frame*), DPX_CHANNEL_QUEUE_HWM);
	ptr->outgoing = alchancreate(sizeof(dpx_frame*), DPX_CHANNEL_QUEUE_HWM);
	ptr->ocleanup = alchancreate(sizeof(unsigned long*), 1);

	ptr->err = 0;
	ptr->method = NULL;

	return ptr;
}

void _dpx_channel_free(dpx_channel* c) {
	_dpx_channel_close(c, DPX_ERROR_FREEING);

	alchanfree(c->connCh);
	alchanfree(c->incoming);
	alchanfree(c->outgoing);
	alchanfree(c->ocleanup);

	free(c->lock);
	free(c->method);
	free(c);
}

dpx_channel* _dpx_channel_new_client(dpx_peer *p, char* method) {
	dpx_channel* ptr = _dpx_channel_new();
	ptr->peer = p;
	ptr->id = p->chanIndex;

	ptr->method = malloc(strlen(method) + 1);
	strcpy(ptr->method, method);

	p->chanIndex += 1;

	taskcreate(&_dpx_channel_pump_outgoing, ptr, DPX_TASK_STACK_SIZE);
	return ptr;
}

dpx_channel* _dpx_channel_new_server(dpx_duplex_conn *conn, dpx_frame *frame) {
	dpx_channel* ptr = _dpx_channel_new();

	ptr->server = 1;
	ptr->peer = conn->peer;

	ptr->id = frame->channel;

	char* methodcpy = (char*) malloc(strlen(frame->method) + 1);
	strcpy(methodcpy, frame->method);
	ptr->method = methodcpy;

	taskcreate(&_dpx_channel_pump_outgoing, ptr, DPX_TASK_STACK_SIZE);
	_dpx_duplex_conn_link_channel(conn, ptr);
	return ptr;
}

void _dpx_channel_close(dpx_channel *c, DPX_ERROR err) {
	qlock(c->lock);

	if (c->closed) {
		qunlock(c->lock);
		return;
	}

	alchanclose(c->connCh);
	alchanclose(c->incoming);
	alchanclose(c->outgoing);

	c->closed = 1;
	c->err = err;

	_dpx_duplex_conn_unlink_channel(c->conn, c);

	qunlock(c->lock);

	alchanrecvul(c->ocleanup);
	alchanclose(c->ocleanup);

}

struct _dpx_channel_close_via_task_struct {
	dpx_channel *c;
	DPX_ERROR err;
};

void _dpx_channel_close_via_task(struct _dpx_channel_close_via_task_struct *s) {
	taskname("_dpx_channel_close_via_task_%d", s->c->id);
	_dpx_channel_close(s->c, s->err);
	free(s);
}

DPX_ERROR _dpx_channel_error(dpx_channel *c) {
	qlock(c->lock);
	DPX_ERROR ret = c->err;
	qunlock(c->lock);
	return ret;
}

dpx_frame* _dpx_channel_receive_frame(dpx_channel *c) {
	if (c->server && c->last) {
		return NULL;
	}

	dpx_frame* frame;
	int ok = alchanrecv(c->incoming, &frame);
	// FIXME is cleanup needed on this dpx_frame if not okay? (e.g. free)
	// FIXME YES
	if (ok == ALCHAN_CLOSED) {
		return NULL;
	}

	if (frame->last) {
		if (c->server) {
			c->last = 1;
		} else {
			_dpx_channel_close(c, DPX_ERROR_NONE);
		}
	}

	return frame;
}

DPX_ERROR _dpx_channel_send_frame(dpx_channel *c, dpx_frame *frame) {
	qlock(c->lock);
	DPX_ERROR ret = DPX_ERROR_NONE;

	if (c->err) {
		ret = c->err;
		goto _dpx_channel_send_frame_cleanup;
	}

	if (c->closed) {
		ret = DPX_ERROR_CHAN_CLOSED;
		goto _dpx_channel_send_frame_cleanup;
	}

	dpx_frame *copy = dpx_frame_new();

	dpx_frame_copy(copy, frame);

	copy->chanRef = c;
	copy->channel = c->id;
	copy->type = DPX_FRAME_DATA;

	alchansend(c->outgoing, &copy);

_dpx_channel_send_frame_cleanup:
	qunlock(c->lock);
	return ret;
}

char* _dpx_channel_method_set(dpx_channel *c, char* method) {
	char* ret = c->method;
	c->method = method;
	return ret;
}

char* _dpx_channel_peer(dpx_channel *c) {
	if (c->conn == NULL)
		return NULL;

	uuid_t *uuid = c->conn->uuid;

	char* str_uuid = malloc(UUID_LEN_STR);
	size_t str_len = UUID_LEN_STR;

	uuid_rc_t res = uuid_export(uuid, UUID_LEN_STR, &str_uuid, &str_len);
	assert(res == UUID_RC_OK);

	return str_uuid;
}

int _dpx_channel_handle_incoming(dpx_channel *c, dpx_frame *frame) {
	qlock(c->lock);

	int ret = 0;

	if (c->closed) {
		goto _dpx_channel_handle_incoming_cleanup;
	}

	if (!frame->last && frame->error != NULL && strcmp(frame->error, "") != 0) {
		struct _dpx_channel_close_via_task_struct *s = malloc(sizeof(struct _dpx_channel_close_via_task_struct));
		s->c = c;
		s->err = DPX_ERROR_CHAN_FRAME;
		taskcreate(&_dpx_channel_close_via_task, s, DPX_TASK_STACK_SIZE);
		ret = 1;
		goto _dpx_channel_handle_incoming_cleanup;
	}

	alchansend(c->incoming, &frame);
	ret = 1;

_dpx_channel_handle_incoming_cleanup:
	qunlock(c->lock);
	return ret;
}

void _dpx_channel_pump_outgoing(dpx_channel *c) {
	taskname("_dpx_channel_pump_outgoing [%d/%d]", c->peer->index, c->id);

	DEBUG_FUNC(printf("(%d) Pumping started for channel %d\n", c->peer->index, c->id));
	alchanrecv(c->connCh, &c->conn);
	DEBUG_FUNC(printf("(%d) Received initial connection for channel %d\n", c->peer->index, c->id));

	while(1) {

		dpx_duplex_conn *conn;
		int hasConn = alchannbrecv(c->connCh, &conn);
		if (hasConn == ALCHAN_CLOSED)
			goto _dpx_channel_pump_outgoing_cleanup;
		else if (hasConn != ALCHAN_NONE)
			c->conn = conn;

		dpx_frame *frame;
		int hasFrame = alchannbrecv(c->outgoing, &frame);
		if (hasFrame == ALCHAN_CLOSED)
			goto _dpx_channel_pump_outgoing_cleanup;
		else if (hasFrame != ALCHAN_NONE) {
			while(1) {
				DEBUG_FUNC(printf("(%d) Sending data frame for channel %d: %d bytes\n", c->peer->index, c->id, frame->payloadSize));
				DPX_ERROR err = _dpx_duplex_conn_write_frame(c->conn, frame);
				if (err) {
					fprintf(stderr, "(%d) Error sending frame: %lu\n", c->peer->index, err);

					if (alchanrecv(c->connCh, &c->conn) == ALCHAN_CLOSED)
						goto _dpx_channel_pump_outgoing_cleanup;

					continue;
				}
				if (frame->error != NULL && strcmp(frame->error, "")) {
					_dpx_channel_close(c, DPX_ERROR_CHAN_FRAME);
				} else if (frame->last && c->server) {
					_dpx_channel_close(c, DPX_ERROR_NONE);
				}
				DEBUG_FUNC(printf("(%d) Sent data frame for channel %d: %d bytes\n", c->peer->index, c->id, frame->payloadSize));
				break;
			}
			free(frame->payload);
			dpx_frame_free(frame);
		}

		if (!c->closed) {
			taskdelay(0); // FIXME investigate why taskyield doesn't work
		}
	}

_dpx_channel_pump_outgoing_cleanup:
	DEBUG_FUNC(printf("(%d) Pumping finished for channel %d\n", c->peer->index, c->id));
	c->conn = NULL;

	alchansendul(c->ocleanup, 100);
	taskexit(0);
}