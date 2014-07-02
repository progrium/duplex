#include "dpx-internal.h"

int _dpx_peer_index = 0;
char byte = '\r';

void* _dpx_peer_free_helper(void *v) {
	_dpx_peer_free((dpx_peer*) v);
	return NULL;
}

void dpx_peer_free(dpx_peer *p) {
	_dpx_a a;
	a.function = &_dpx_peer_free_helper;
	a.args = p;

	assert(_dpx_joinfunc(&a) == NULL);
}

void* _dpx_peer_new_helper(void *v) {
	assert(v == NULL);
	return _dpx_peer_new();
}

dpx_peer* dpx_peer_new() {
	_dpx_a a;
	a.function = &_dpx_peer_new_helper;
	a.args = NULL;

	void* peer = _dpx_joinfunc(&a);
	return (dpx_peer*)peer;
}

struct _dpx_peer_open_hs {
	dpx_peer *p;
	char* method;
};

void* _dpx_peer_open_helper(void* v) {
	struct _dpx_peer_open_hs *h = (struct _dpx_peer_open_hs*) v;
	return _dpx_peer_open(h->p, h->method);
}

dpx_channel* dpx_peer_open(dpx_peer *p, char *method) {
	_dpx_a a;
	a.function = &_dpx_peer_open_helper;

	struct _dpx_peer_open_hs h;
	h.p = p;
	h.method = method;

	a.args = &h;

	void* res = _dpx_joinfunc(&a);
	return (dpx_channel*) res;
}

struct _dpx_peer_handle_open_hs {
	dpx_peer *p;
	dpx_duplex_conn *conn;
	dpx_frame *frame;
	int ret;
};

void* _dpx_peer_handle_open_helper(void* v) {
	struct _dpx_peer_handle_open_hs *h = (struct _dpx_peer_handle_open_hs*) v;
	h->ret = _dpx_peer_handle_open(h->p, h->conn, h->frame);
	return NULL;
}

int dpx_peer_handle_open(dpx_peer *p, dpx_duplex_conn *conn, dpx_frame *frame) {
	_dpx_a a;
	a.function = &_dpx_peer_handle_open_helper;

	struct _dpx_peer_handle_open_hs h;
	h.p = p;
	h.conn = conn;
	h.frame = frame;

	a.args = &h;

	_dpx_joinfunc(&a);
	
	return h.ret;
}

void* _dpx_peer_accept_helper(void* v) {
	dpx_peer *p = (dpx_peer*) v;
	dpx_channel* ch = _dpx_peer_accept(p);
	return ch;
}

dpx_channel* dpx_peer_accept(dpx_peer *p) {
	_dpx_a a;
	a.function = &_dpx_peer_accept_helper;
	a.args = p;

	void* ret = _dpx_joinfunc(&a);
	
	return (dpx_channel*)ret;
}

struct _dpx_peer_close_hs {
	dpx_peer *p;
	DPX_ERROR err;
};

void* _dpx_peer_close_helper(void* v) {
	struct _dpx_peer_close_hs *h = (struct _dpx_peer_close_hs*) v;
	h->err = _dpx_peer_close(h->p);
	return NULL;
}

DPX_ERROR dpx_peer_close(dpx_peer *p) {
	_dpx_a a;
	a.function = &_dpx_peer_close_helper;

	struct _dpx_peer_close_hs h;
	h.p = p;

	a.args = &h;

	_dpx_joinfunc(&a);
	
	return h.err;
}

struct _dpx_peer_conbind_hs {
	dpx_peer *p;
	char* addr;
	int port;
	DPX_ERROR err;
};

void* _dpx_peer_connect_helper(void* v) {
	struct _dpx_peer_conbind_hs *h = (struct _dpx_peer_conbind_hs*)v;
	h->err = _dpx_peer_connect(h->p, h->addr, h->port);
	return NULL;
}

DPX_ERROR dpx_peer_connect(dpx_peer *p, char* addr, int port) {
	_dpx_a a;
	a.function = &_dpx_peer_connect_helper;

	struct _dpx_peer_conbind_hs h;
	h.p = p;
	h.addr = addr;
	h.port = port;

	a.args = &h;

	_dpx_joinfunc(&a);
	
	return h.err;
}

void* _dpx_peer_bind_helper(void* v) {
	struct _dpx_peer_conbind_hs *h = (struct _dpx_peer_conbind_hs*)v;
	h->err = _dpx_peer_bind(h->p, h->addr, h->port);
	return NULL;
}

DPX_ERROR dpx_peer_bind(dpx_peer *p, char* addr, int port) {
	_dpx_a a;
	a.function = &_dpx_peer_bind_helper;

	struct _dpx_peer_conbind_hs h;
	h.p = p;
	h.addr = addr;
	h.port = port;

	a.args = &h;

	_dpx_joinfunc(&a);
	
	return h.err;
}


// ----------------------------------------------------------------------------

void _dpx_peer_free(dpx_peer *p) {
	free(p->lock);

	dpx_peer_listener *l, *nl;
	for (l=p->listeners; l != NULL; l=nl) {
		nl = l->next;
		free(l);
	}

	dpx_peer_connection *c, *nc;
	for (c=p->conns; c != NULL; c=nc) {
		nc = c->next;
		free(c);
	}

	if (p->openFrames != NULL)
		chanfree(p->openFrames);
	if (p->incomingChannels != NULL)
		chanfree(p->incomingChannels);
	chanfree(p->firstConn);
	free(p);
}

// FIXME this isn't threadsafe (taskcreate is called)
dpx_peer* _dpx_peer_new() {
	dpx_peer* peer = (dpx_peer*) malloc(sizeof(dpx_peer));

	peer->lock = (QLock*) calloc(1, sizeof(QLock));

	peer->listeners = NULL;
	peer->conns = NULL;

	peer->openFrames = chancreate(sizeof(dpx_frame), DPX_CHANNEL_QUEUE_HWM);
	peer->incomingChannels = chancreate(sizeof(dpx_channel), 1024);

	peer->closed = 0;
	peer->rrIndex = 0;
	peer->chanIndex = 0;
	peer->index = 0;

	peer->firstConn = chancreate(sizeof(char), 0);

	_dpx_peer_index += 1;

	taskcreate(&_dpx_peer_route_open_frames, peer, DPX_TASK_STACK_SIZE);
	return peer;
}

void _dpx_peer_accept_connection(dpx_peer *p, int fd) {
	dpx_duplex_conn* dc = _dpx_duplex_conn_new(p, fd);
	qlock(p->lock);

	dpx_peer_connection* add = (dpx_peer_connection*) malloc(sizeof(dpx_peer_connection));
	add->conn = dc;
	add->next = NULL;

	dpx_peer_connection* conn = p->conns;
	if (conn == NULL) {
		p->conns = add;
		chansend(p->firstConn, &byte);
	} else {
		while (conn->next != NULL)
			conn = conn->next;
		conn->next = add;
	}

	qunlock(p->lock);

	taskcreate(&_dpx_duplex_conn_read_frames, dc, DPX_TASK_STACK_SIZE);
	taskcreate(&_dpx_duplex_conn_write_frames, dc, DPX_TASK_STACK_SIZE);
}

int _dpx_peer_next_conn(dpx_peer *p, dpx_duplex_conn **conn) {
	int connlen = 0;
	dpx_peer_connection *tmp;
	for (tmp=p->conns; tmp != NULL; tmp=tmp->next)
		connlen++;

	assert(connlen != 0);

	int index = p->rrIndex % connlen;
	qlock(p->lock);

	int i = 0;
	for (tmp=p->conns; i < index; tmp=tmp->next)
		i++;

	p->rrIndex++;

	qunlock(p->lock);

	*conn = tmp->conn;
	return index;
}

int _dpx_peer_connlen(dpx_peer *p) {
	int connlen = 0;
	dpx_peer_connection *tmp;
	for (tmp=p->conns; tmp != NULL; tmp=tmp->next)
		connlen++;
	return connlen;
}

void _dpx_peer_route_open_frames(dpx_peer *p) {
	DPX_ERROR err = DPX_ERROR_NONE;
	dpx_frame* frame = NULL;

	while(1) {
		chanrecvp(p->firstConn); // we don't care about the ret value
		printf("First connection, routing... [index %d]\n", p->index);

		while (_dpx_peer_connlen(p) > 0) {
			if (err == DPX_ERROR_NONE) {
				if (p->openFrames == NULL)
					return;
				chanrecv(p->openFrames, &frame);
			}
			dpx_duplex_conn* conn;
			int index = _dpx_peer_next_conn(p, &conn);
			printf("Sending frame [%d]: %d bytes\n", index, frame->payloadSize);
			err = _dpx_duplex_conn_write_frame(conn, frame);
			if (err == DPX_ERROR_NONE)
				_dpx_duplex_conn_link_channel(conn, frame->chanRef);
		}
	}
}

dpx_channel* _dpx_peer_open(dpx_peer *p, char *method) {
	qlock(p->lock);
	dpx_channel* ret = NULL;

	if (p->closed)
		goto _dpx_peer_open_cleanup;

	ret = _dpx_channel_new_client(p, method);
	dpx_frame* frame = _dpx_frame_new(ret);

	frame->type = DPX_FRAME_OPEN;
	frame->method = method;

	chansend(p->openFrames, frame);

_dpx_peer_open_cleanup:
	qunlock(p->lock);
	return ret;
}

int _dpx_peer_handle_open(dpx_peer *p, dpx_duplex_conn *conn, dpx_frame *frame) {
	qlock(p->lock);
	int ret = 0;

	if (p->closed)
		goto _dpx_peer_handle_open_cleanup;

	chansend(p->incomingChannels, _dpx_channel_new_server(conn, frame));
	ret = 1;

_dpx_peer_handle_open_cleanup:
	qunlock(p->lock);
	return ret;
}

dpx_channel* _dpx_peer_accept(dpx_peer *p) {
	if (p->incomingChannels == NULL)
		return NULL;
	// FIXME we can't detect a channel we close... or can we?
	return chanrecvp(p->incomingChannels);
}

DPX_ERROR _dpx_peer_close(dpx_peer *p) {
	qlock(p->lock);
	DPX_ERROR ret = DPX_ERROR_NONE;

	if (p->closed) {
		ret = DPX_ERROR_PEER_ALREADYCLOSED;
		goto _dpx_peer_close_cleanup;
	}

	p->closed = 1;

	chanfree(p->openFrames);
	p->openFrames = NULL;

	chanfree(p->incomingChannels);
	p->incomingChannels = NULL;

	dpx_peer_listener *l, *nl;
	for (l=p->listeners; l != NULL; l=nl) {
		close(l->fd);
		nl = l->next;
		free(l);
	}

	dpx_peer_connection *c, *nc;
	for (c=p->conns; c != NULL; c=nc) {
		_dpx_duplex_conn_close(c->conn);
		_dpx_duplex_conn_free(c->conn);
		nc = c->next;
		free(c);
	}

_dpx_peer_close_cleanup:
	qunlock(p->lock);
	return ret;
}

struct _dpx_peer_connect_task_param {
	dpx_peer *p;
	char* addr;
	int port;
};

DPX_ERROR _dpx_peer_send_greeting(int connfd) {
	// TODO
	return DPX_ERROR_NONE;
}

int _dpx_peer_receive_greeting(int connfd) {
	// TODO
	return 1;
}

void _dpx_peer_connect_task(struct _dpx_peer_connect_task_param *param) {
	dpx_peer *p = param->p;
	char* addr = param->addr;
	int port = param->port;

	int i;
	for (i=0; i < DPX_PEER_RETRYATTEMPTS; i++) {
		printf("(%d) Connecting to %s:%d\n", p->index, addr, port);
		int connfd = netdial(TCP, addr, port);
		if (connfd < 0) {
			printf("(%d) Failed to connect... Attempt %d/%d.\n", p->index, i+1, DPX_PEER_RETRYATTEMPTS);
			taskdelay(DPX_PEER_RETRYMS);
			continue;
		}
		if (_dpx_peer_send_greeting(connfd) != DPX_ERROR_NONE) {
			printf("(%d) Failed to make greeting...\n", p->index);
			close(connfd);
			goto _dpx_peer_connect_task_cleanup;
		}
		printf("(%d) Connected.\n", p->index);
		_dpx_peer_accept_connection(p, connfd);
		goto _dpx_peer_connect_task_cleanup;
	}

_dpx_peer_connect_task_cleanup:
	free(param);
}

DPX_ERROR _dpx_peer_connect(dpx_peer *p, char* addr, int port) {
	qlock(p->lock);
	DPX_ERROR ret = DPX_ERROR_NONE;

	if (p->closed) {
		ret = DPX_ERROR_PEER_ALREADYCLOSED;
		goto _dpx_peer_connect_cleanup;
	}

	struct _dpx_peer_connect_task_param *param = (struct _dpx_peer_connect_task_param*) malloc(sizeof(struct _dpx_peer_connect_task_param));
	param->p = p;
	param->addr = addr;
	param->port = port;

	taskcreate(&_dpx_peer_connect_task, param, DPX_TASK_STACK_SIZE);

_dpx_peer_connect_cleanup:
	qunlock(p->lock);
	return ret;
}

struct _dpx_peer_bind_task_param {
	dpx_peer *p;
	int connfd;
};

void _dpx_peer_bind_task_accept(struct _dpx_peer_bind_task_param *param) {
	if (_dpx_peer_receive_greeting(param->connfd)) {
		_dpx_peer_accept_connection(param->p, param->connfd);
	}

	free(param);
}

void _dpx_peer_bind_task(struct _dpx_peer_bind_task_param *param) {
	dpx_peer *p = param->p;
	int connfd = param->connfd;

	while(1) {
		char server[16];
		int port;
		int fd = netaccept(connfd, server, &port);
		if (fd < 0) {
			printf("failed to receive connection...\n");
			break;
		}
		printf("accepted connection from %.*s:%d\n", 16, server, port);
		struct _dpx_peer_bind_task_param *ap = (struct _dpx_peer_bind_task_param*) malloc(sizeof(struct _dpx_peer_bind_task_param));
		ap->p = p;
		ap->connfd = fd;
		taskcreate(&_dpx_peer_bind_task_accept, ap, DPX_TASK_STACK_SIZE);
	}

	free(param);
}

DPX_ERROR _dpx_peer_bind(dpx_peer *p, char* addr, int port) {
	qlock(p->lock);
	DPX_ERROR ret = DPX_ERROR_NONE;

	if (p->closed) {
		ret = DPX_ERROR_PEER_ALREADYCLOSED;
		goto _dpx_peer_bind_cleanup;
	}

	int listener = netannounce(TCP, addr, port);
	if (listener < 0) {
		ret = DPX_ERROR_NETWORK_FAIL;
		goto _dpx_peer_bind_cleanup;
	}

	dpx_peer_listener *add = (dpx_peer_listener*) malloc(sizeof(dpx_peer_listener));
	add->fd = listener;
	add->next = NULL;

	dpx_peer_listener *l = p->listeners;
	if (l == NULL) {
		p->listeners = add;
	} else {
		while (l->next != NULL)
			l = l->next;
		l->next = add;
	}

	printf("Now listening on %s:%d\n", addr, port);

	struct _dpx_peer_bind_task_param *param = (struct _dpx_peer_bind_task_param*) malloc(sizeof(struct _dpx_peer_bind_task_param));
	param->p = p;
	param->connfd = listener;

	taskcreate(&_dpx_peer_bind_task, param, DPX_TASK_STACK_SIZE);

_dpx_peer_bind_cleanup:
	qunlock(p->lock);
	return ret;
}
