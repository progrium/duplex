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
			msgpack_object_array arr = obj.via.array;
			// create frame
			dpx_frame* frame = dpx_frame_new(NULL);

			msgpack_object* o = arr.ptr;
			frame->type = o.via.i64;
			o++;

			frame->channel = o.via.i64;
			o++;

			char* methodBuf = (char*) malloc(o.via.raw.size+1);
			strncpy(o.via.raw.ptr, methodBuf, o.via.raw.size);
			*(methodBuf + o.via.raw.size) = '\0';
			frame->method = methodBuf;
			o++;

			msgpack_object_map headers_map = o.via.map;
			uint32_t headers_map_size = o.via.map.size;
			msgpack_object_kv* headers = o.via.map.ptr;
			o++;
			// go through kv and add to hashtable

			int i;

			for (i=0; i<headers_map_size; i++) {
				dpx_header_map* header = (dpx_header_map*) malloc(sizeof(dpx_header_map));
				strcpy(headers->key->via->raw->ptr, header->key);
				strcpy(headers->val->via->raw->ptr, header->value);
				HASH_ADD_KEYPTR(hh, frame->headers, header->key, strlen(header->key), header);
				headers++;
			}

			char* errorBuf = (char*) malloc(o.via.raw.size+1);
			strncpy(o.via.raw, errorBuf, o.via.raw.size);
			*(errorBuf + o.via.raw.size) = '\0';
			frame->error = errorBuf;
			o++;

			frame->last = o.via.i64;
			o++;

			char* payloadBuf = (char*) malloc(o.via.raw.size);
			strncpy(o.via.raw, payloadBuf, o.via.raw.size);
			frame->payload = payloadBuf;

			frame->payloadSize = o.via.raw.size;

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