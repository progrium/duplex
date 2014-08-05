#include "dpx-internal.h"
#include <string.h>

dpx_duplex_conn* _dpx_duplex_conn_new(dpx_peer *p, int fd) {
	dpx_duplex_conn* c = (dpx_duplex_conn*) malloc(sizeof(dpx_duplex_conn));

	c->lock = calloc(1, sizeof(QLock));
	c->peer = p;
	c->connfd = fd;
	fdnoblock(c->connfd);
	c->writeCh = alchancreate(sizeof(dpx_frame*), 0);
	c->channels = NULL;

	return c;
}

void _dpx_duplex_conn_close(dpx_duplex_conn *c) {
	alchanclose(c->writeCh);
}

void _dpx_duplex_conn_free(dpx_duplex_conn *c) {
	free(c->lock);
	alchanfree(c->writeCh);

	dpx_channel_map *current, *tmp;
	HASH_ITER(hh, c->channels, current, tmp) {
		HASH_DEL(c->channels, current);
		free(current);
	}

	free(c);
}

void _dpx_duplex_conn_read_frames(void *v) {
	dpx_duplex_conn *c = v;

	taskname("_dpx_duplex_conn_read_frames (peer %d)", c->peer->index);
	// FIXME make sure conn is open (see libtask netaccept and fdopen())

	char buf[DPX_DUPLEX_CONN_CHUNK];
	ssize_t read_size;

	msgpack_unpacker unpacker; // does the unpacking
	msgpack_unpacked result; // gets the result

	msgpack_unpacker_init(&unpacker, DPX_DUPLEX_CONN_BUFFER);
	msgpack_unpacked_init(&result);

	while((read_size = fdread(c->connfd, buf, DPX_DUPLEX_CONN_CHUNK)) > 0) {
		msgpack_unpacker_reserve_buffer(&unpacker, read_size);
		memcpy(msgpack_unpacker_buffer(&unpacker), buf, read_size);
		msgpack_unpacker_buffer_consumed(&unpacker, read_size);
		while (msgpack_unpacker_next(&unpacker, &result)) {
			// result is here!
			msgpack_object obj = result.data;

			// make sure it's an array
			assert (obj.type == MSGPACK_OBJECT_ARRAY);

			dpx_frame* frame = _dpx_frame_msgpack_from(&obj);

			// got the frame, now save it.
			qlock(c->lock);
			dpx_channel_map *channel = NULL;

			HASH_FIND_INT(c->channels, &frame->channel, channel);

			qunlock(c->lock);

			DEBUG_FUNC(printf("channel = %p, frame->channel = %d, frame->type = %d\n", channel, frame->channel, frame->type));
			if (channel != NULL && frame->type == DPX_FRAME_DATA) {
				if (_dpx_channel_handle_incoming(channel->value, frame))
					continue;
			}
			if (channel == NULL && frame->type == DPX_FRAME_OPEN) {
				if (_dpx_peer_handle_open(c->peer, c, frame)) {
					free(frame->payload);
					dpx_frame_free(frame);
					continue;
				}
			}
			fprintf(stderr, "(%d) dropped frame, size %d", c->peer->index, frame->payloadSize);
			free(frame->payload);
			dpx_frame_free(frame);
		}
	}

	_dpx_duplex_conn_close(c);
}

void _dpx_duplex_conn_write_frames(dpx_duplex_conn *c) {
	taskname("_dpx_duplex_conn_write_frames (peer %d)", c->peer->index);

	while(1) {
		dpx_frame* frame;
		if (alchanrecv(c->writeCh, &frame) == ALCHAN_CLOSED)
			return;

		msgpack_sbuffer* encoded = _dpx_frame_msgpack_to(frame);
		ssize_t result = fdwrite(c->connfd, encoded->data, encoded->size);

		if (result < 0) {
			alchansendul(frame->errCh, DPX_ERROR_NETWORK_FAIL);
			fprintf(stderr, "(%d) Sending frame failed due to system error: %zu bytes\n", c->peer->index, encoded->size);
		} else if (result != encoded->size) {
			alchansendul(frame->errCh, DPX_ERROR_NETWORK_NOTALL);
			fprintf(stderr, "(%d) Sending frame failed because not all bytes were sent: %zu/%zu bytes\n", c->peer->index, result, encoded->size);
		} else {
			alchansendul(frame->errCh, DPX_ERROR_NONE);
		}

		msgpack_sbuffer_free(encoded);
	}

	// FIXME FIXME FIXME FIXME FIXME
	// cannot break above because read_frames will fail
	close(c->connfd);
}

DPX_ERROR _dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame) {
	frame->errCh = alchancreate(sizeof(DPX_ERROR), 0);

	DPX_ERROR result = DPX_ERROR_NONE;

	if (alchansend(c->writeCh, &frame) == ALCHAN_CLOSED) {
		result = DPX_ERROR_DUPLEX_CLOSED;
		goto _dpx_duplex_conn_write_frame_cleanup;
	}

	result = alchanrecvul(frame->errCh);

_dpx_duplex_conn_write_frame_cleanup:
	alchanclose(frame->errCh);
	assert(alchanfree(frame->errCh));
	frame->errCh = NULL;

	return result;
}

void _dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch) {
	qlock(c->lock);
	dpx_channel_map *insert = malloc(sizeof(dpx_channel_map));
	insert->key = ch->id;
	insert->value = ch;

	dpx_channel_map *old;
	HASH_REPLACE_INT(c->channels, key, insert, old);
	if (old != NULL)
		free(old);

	alchansend(ch->connCh, &c);
	qunlock(c->lock);
}

void _dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch) {
	qlock(c->lock);
	dpx_channel_map *m;

	HASH_FIND_INT(c->channels, &ch->id, m);

	if (m != NULL)
		HASH_DEL(c->channels, m);

	qunlock(c->lock);
}