#include "dpx-c.h"
#include <string.h>

void dpx_duplex_conn_read_frames(dpx_duplex_conn *c) {
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

			dpx_frame* frame = dpx_frame_msgpack_from(&obj);

			// got the frame, now save it.
			qlock(c->lock);
			dpx_channel_map *channel = NULL;

			HASH_FIND_INT(c->channels, &frame->channel, channel);

			qunlock(c->lock);
			if (channel != NULL && frame->type == DPX_FRAME_DATA) {
				if (dpx_channel_handle_incoming(channel->value, frame))
					continue;
			}
			if (channel == NULL && frame->type == DPX_FRAME_OPEN) {
				if (dpx_peer_handle_open(c->peer, frame))
					continue;
			}
			printf("dropped frame, size %d", frame->payloadSize);
			dpx_frame_free(frame);
		}
	}

	// FIXME close(c.writeCh)
}

void dpx_duplex_conn_write_frames(dpx_duplex_conn *c) {
	while(1) {
		if (c->writeCh == NULL)
			return;

		dpx_frame* frame;
		if (!chanrecv(c->writeCh, frame))
			return;

		msgpack_sbuffer* encoded = dpx_frame_msgpack_to(frame);
		int result = fdwrite(c->connfd, encoded->data, encoded->size);

		if (result < 0) {
			chansendul(frame->errCh, DPX_ERROR_NETWORK_FAIL);
			printf("Sending frame failed due to system error: %d bytes\n", encoded->size);
			return;
		} else if (result != encoded->size) {
			chansendul(frame->errCh, DPX_ERROR_NETWORK_NOTALL);
			printf("Sending frame failed because not all bytes were sent: %d/%d bytes\n", result, encoded->size);
			return;
		} else {
			chansendul(frame->errCh, DPX_ERROR_NONE);
		}
	}
}

DPX_ERROR dpx_duplex_conn_write_frame(dpx_duplex_conn *c, dpx_frame *frame) {
	chansend(c->writeCh, frame);
	return chanrecvul(frame->errCh);
}

void dpx_duplex_conn_link_channel(dpx_duplex_conn *c, dpx_channel* ch) {
	qlock(c->lock);
	dpx_channel_map* ifany = HASH_REPLACE_INT(c->channels, key, c);
	if (ifany != NULL)
		free(ifany);

	chansend(ch->connCh, c);
	qunlock(c->lock);
}

void dpx_duplex_conn_unlink_channel(dpx_duplex_conn *c, dpx_channel* ch) {
	qlock(c->lock);
	HASH_DEL(c->channels, ch);
	qunlock(c->lock);
}