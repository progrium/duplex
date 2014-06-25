#include "dpx-c.h"
#include <string.h> // for strcmp

dpx_channel* _dpx_channel_new() {
	dpx_channel* ptr = (dpx_channel*) malloc(sizeof(dpx_channel));

	ptr->lock = (QLock*) calloc(sizeof(QLock));
	ptr->id = 0;

	ptr->peer = NULL;
	ptr->connCh = chancreate(sizeof(dpx_duplex_conn*), 0);
	ptr->conn = NULL;

	ptr->server = 0;
	ptr->closed = 0;
	ptr->last = 0;

	ptr->incoming = chancreate(sizeof(dpx_frame*), DPX_CHANNEL_QUEUE_HWM);
	ptr->outgoing = chancreate(sizeof(dpx_frame*), DPX_CHANNEL_QUEUE_HWM);

	ptr->err = 0;
	ptr->method = NULL;

	return ptr;
}

void dpx_channel_free(dpx_channel* c) {
	dpx_channel_close(c, DPX_ERROR_FREEING);
	free(c);
}

dpx_channel* dpx_channel_new_client(dpx_peer *p, char* method) {
	dpx_channel* ptr = _dpx_channel_new();
	ptr->peer = p;
	ptr->id = p->chanIndex;
	ptr->method = method;

	p->chanIndex += 1;

	taskcreate(&dpx_channel_pump_outgoing, ptr, DPX_TASK_STACK_SIZE);
	return ptr;
}

dpx_channel* dpx_channel_new_server(dpx_duplex_conn *conn, dpx_frame *frame) {
	dpx_channel* ptr = _dpx_channel_new();
	
	ptr->server = 1;
	ptr->peer = conn->peer;

	ptr->id = frame->channel;
	ptr->method = frame->method;

	taskcreate(&dpx_channel_pump_outgoing, ptr, DPX_TASK_STACK_SIZE);
	dpx_duplex_conn_link_channel(conn, ptr);
	return ptr;
}

void dpx_channel_close(dpx_channel *c, DPX_ERROR err) {
	qlock(c->lock);

	if (c->closed) {
		goto dpx_channel_close_cleanup;
	}
	c->closed = 1;
	c->err = err;

	dpx_duplex_conn_unlink_channel(c->conn, c);

	chanfree(c->connCh);
	c->connCh = NULL;
	chanfree(c->incoming);
	c->incoming = NULL;
	chanfree(c->outgoing);
	c->outgoing = NULL;

dpx_channel_close_cleanup:
	qunlock(c->lock);
}

struct _dpx_channel_close_via_task_struct {
	dpx_channel *c;
	DPX_ERROR err;
};

void dpx_channel_close_via_task(struct _dpx_channel_close_via_task_struct *s) {
	dpx_channel_close(s->c, s->err);
	free(s);
}

DPX_ERROR dpx_channel_error(dpx_channel *c) {
	qlock(c->lock);
	DPX_ERROR ret = c->err;
	qunlock(c->lock);
	return ret;
}

dpx_frame* dpx_channel_receive_frame(dpx_channel *c) {
	if (c->server && c->last) {
		return NULL;
	}

	dpx_frame* frame;
	int ok = chanrecv(c->incoming, &frame);
	// FIXME is cleanup needed on this dpx_frame if not okay? (e.g. free)
	if (!ok) {
		return NULL;
	}

	if (frame->last) {
		if (c->server) {
			c->last = 1;
		} else {
			dpx_channel_close(c, DPX_ERROR_NONE);
		}
	}

	return frame;
}

DPX_ERROR dpx_channel_send_frame(dpx_channel *c, dpx_frame *frame) {
	qlock(c->lock);
	DPX_ERROR ret = DPX_ERROR_NONE;

	// FIXME if err != NONE, then the caller might have to free frame

	if (c->err) {
		ret = c->err;
		goto dpx_channel_send_frame_cleanup;
	}

	if (c->closed) {
		ret = DPX_ERROR_CHAN_CLOSED;
		goto dpx_channel_send_frame_cleanup;
	}

	frame->chanRef = c;
	frame->channel = c->id;
	frame->type = DPX_FRAME_DATA;
	chansend(c->outgoing, frame);

dpx_channel_send_frame_cleanup:
	qunlock(c->lock);
	return ret;
}

int dpx_channel_handle_incoming(dpx_channel *c, dpx_frame *frame) {
	qlock(c->lock);

	int ret = 0;

	if (c->closed) {
		goto dpx_channel_handle_incoming_cleanup;
	}

	if (!frame->last && strcmp(frame->error, "") != 0) {
		struct _dpx_channel_close_via_task_struct *s = (struct _dpx_channel_close_via_task_struct*) malloc(sizeof(struct _dpx_channel_close_via_task_struct));
		s->c = c;
		s->err = DPX_ERROR_CHAN_FRAME;
		taskcreate(&dpx_channel_close_via_task, s, DPX_TASK_STACK_SIZE);
		ret = 1;
		goto dpx_channel_handle_incoming_cleanup;
	}

	chansend(c->incoming, frame);
	ret = 1;

dpx_channel_handle_incoming_cleanup:
	qunlock(c->unlock);
	return ret;
}

void dpx_channel_pump_outgoing(dpx_channel *c) {
	int stillopen = 0;
	chanrecv(c->connCh, c->conn);
	printf("(%d) Pumping started for channel %d\n", c->peer->index, c->id);
	while(1) {
		taskyield();

		if (c->connCh == NULL) {
			c->conn = NULL;
			goto dpx_channel_pump_outgoing_cleanup;
		}
		dpx_duplex_conn* conn;
		int hasConn = channbrecv(c->connCh, conn);
		if (hasConn != -1) {
			c->conn = conn;
		}

		dpx_frame *frame;
		if (c->outgoing == NULL) {
			goto dpx_channel_pump_outgoing_cleanup;
		}
		int hasFrame = channbrecv(c->outgoing, frame);
		if (hasFrame != -1) {
			while(1) {
				printf("(%d) Sending frame: %d bytes\n", c->peer->index, frame->payloadSize);
				DPX_ERROR err = dpx_duplex_conn_write_frame(c->conn, frame);
				if (err) {
					printf("(%d) Error sending frame: %d\n", err);

					// check if we have a new connection...
					if (c->connCh == NULL) {
						// nope, our channel is closed
						c->conn = NULL;
						goto dpx_channel_pump_outgoing_cleanup;
					}

					chanrecv(c->connCh, c->conn);
					continue
				}
				if (strcmp(frame->error, "")) {
					dpx_channel_close(DPX_ERROR_CHAN_FRAME);
				} else if (frame->last && c->server) {
					dpx_channel_close(DPX_ERROR_NONE);
				}
				break;
			}
		}
	}

dpx_channel_pump_outgoing_cleanup:
	printf("(%d) Pumping finished for channel %d\n", c->peer->index, c->id);
}