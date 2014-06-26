#include "dpx-c.h"

void dpx_frame_free(dpx_frame *frame) {
	chanfree(frame->errCh);
	free(frame->method);
	free(frame->error);
	dpx_header_map *current, *tmp;
	HASH_ITER(hh, frame->headers, current, tmp) {
		HASH_DEL(frame->headers, current);
		free(current);
	}
	free(frame);
}

dpx_frame* dpx_frame_new(dpx_channel *ch) {
	dpx_frame *frame = (dpx_frame*) malloc(sizeof(dpx_frame));

	frame->errCh = chancreate(sizeof(DPX_ERROR), 0);
	frame->chanRef = ch;

	frame->type = 0;
	if (ch != NULL)
		frame->channel = ch->id;
	else
		frame->channel = DPX_FRAME_NOCH;

	frame->method = NULL;
	frame->headers = NULL;
	frame->error = NULL;
	frame->last = 0;

	frame->payload = NULL;
	frame->payloadSize = 0;

	return frame;
}

dpx_frame* dpx_frame_msgpack_from(msgpack_object *obj) {
	msgpack_object_array arr = obj->via.array;

	assert(arr.size == DPX_PACK_ARRAY_SIZE);

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

		msgpack_object_raw rawKey = headers->key.via.raw;
		char* keyBuf = (char*) malloc(rawKey.size+1);
		strncpy(rawKey.ptr, keyBuf, rawKey.size);
		*(keyBuf+rawKey.size) = '\0';
		header->key = keyBuf;

		msgpack_object_raw rawValue = headers->val.via.raw;
		char* valBuf = (char*) malloc(rawValue.size+1);
		strncpy(rawValue.ptr, valBuf, rawValue.size);
		*(valBuf+rawValue.size) = '\0';
		header->value = valBuf;
		
		HASH_ADD_KEYPTR(hh, frame->headers, header->key, strlen(header->key), header);
		headers++;
	}

	char* errorBuf = (char*) malloc(o.via.raw.size+1);
	strncpy(o.via.raw.ptr, errorBuf, o.via.raw.size);
	*(errorBuf + o.via.raw.size) = '\0';
	frame->error = errorBuf;
	o++;

	frame->last = o.via.i64;
	o++;

	char* payloadBuf = (char*) malloc(o.via.raw.size);
	strncpy(o.via.raw.ptr, payloadBuf, o.via.raw.size);
	frame->payload = payloadBuf;

	frame->payloadSize = o.via.raw.size;
	
	return frame;
}

msgpack_sbuffer* dpx_frame_msgpack_to(dpx_frame *frame) {
	msgpack_sbuffer* buf = msgpack_sbuffer_new();
	msgpack_packer* pack = msgpack_packer_new(buf, msgpack_sbuffer_write);

	msgpack_pack_array(pack, DPX_PACK_ARRAY_SIZE);
	msgpack_pack_int(pack, frame->type);
	msgpack_pack_channel(pack, frame->channel);

	msgpack_pack_raw(pack, strlen(frame->method));
	msgpack_pack_raw_body(pack, frame->method, strlen(frame->method));

	msgpack_pack_map(pack, HASH_COUNT(frame->headers));

	dpx_header_map *h;
	for (h=frame->headers; h != NULL; h=h->hh.next) {
		msgpack_pack_raw(pack, strlen(h->key));
		msgpack_pack_raw_body(pack, h->key, strlen(h->key));

		msgpack_pack_raw(pack, strlen(h->value));
		msgpack_pack_raw_body(pack, h->value, strlen(h->value));
	}

	msgpack_pack_raw(pack, strlen(frame->error));
	msgpack_pack_raw_body(pack, frame->error, strlen(frame->error));

	msgpack_pack_int(frame->last);

	msgpack_pack_raw(pack, frame->payloadSize);
	msgpack_pack_raw_body(pack, frame->payload, frame->payloadSize);

	msgpack_packer_free(pack);
	return buf;
}