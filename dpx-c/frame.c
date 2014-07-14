#include "dpx-internal.h"

void dpx_frame_free(dpx_frame *frame) {
	chanclose(frame->errCh);
	// empty channel
	DPX_ERROR err;
	while (chanrecv(frame->errCh, &err) != LTCHAN_CLOSED) {}

	chanfree(frame->errCh);
	if (frame->method != NULL)
		free(frame->method);
	if (frame->error != NULL)
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

char* dpx_frame_header_add(dpx_frame *frame, char* key, char* value) {
	dpx_header_map *m = malloc(sizeof(dpx_header_map));
	m->key = malloc(strlen(key) + 1);
	strcpy(m->key, key);
	m->value = malloc(strlen(value) + 1);
	strcpy(m->value, value);

	// remove old header if any
	dpx_header_map *old;
	HASH_FIND_STR(frame->headers, key, old);
	if (old != NULL)
		HASH_DEL(frame->headers, old);

	HASH_ADD_KEYPTR(hh, frame->headers, m->key, strlen(m->key), m);

	if (old == NULL)
		return NULL;

	char* oldval = old->value;

	free(old->key);
	free(old);

	return oldval;
}

char* dpx_frame_header_find(dpx_frame *frame, char* key) {
	dpx_header_map *found;
	HASH_FIND_STR(frame->headers, key, found);

	if (found == NULL)
		return NULL;
	return found->value;
}

void dpx_frame_header_iter(dpx_frame *frame, void (*iter_func)(char* k, char* v)) {
	dpx_header_map *cur, *next;

	HASH_ITER(hh, frame->headers, cur, next) {
		iter_func(cur->key, cur->value);
	}
}

unsigned int dpx_frame_header_len(dpx_frame *frame) {
	return HASH_COUNT(frame->headers);
}

char* dpx_frame_header_rm(dpx_frame *frame, char* key) {
	dpx_header_map *found;
	HASH_FIND_STR(frame->headers, key, found);

	if (found == NULL)
		return NULL;

	HASH_DEL(frame->headers, found);
	char* val = found->value;

	free(found->key);
	free(found);
	return val;
}

// ----------------------------------------------------------------------------

dpx_frame* _dpx_frame_msgpack_from(msgpack_object *obj) {
	msgpack_object_print(stdout, *obj);
	puts("");
	msgpack_object_array arr = obj->via.array;

	assert(arr.size == DPX_PACK_ARRAY_SIZE);

	// create frame
	dpx_frame* frame = dpx_frame_new(NULL);

	msgpack_object* o = arr.ptr;

	frame->type = o[0].via.i64;

	frame->channel = o[1].via.i64;

	msgpack_object_raw methodRaw = o[2].via.raw;
	char* methodBuf = (char*) malloc(methodRaw.size+1);
	strncpy(methodBuf, methodRaw.ptr, methodRaw.size);
	*(methodBuf + methodRaw.size) = '\0';
	frame->method = methodBuf;

	msgpack_object_map headers_map = o[3].via.map;
	msgpack_object_kv* headers = headers_map.ptr;

	// go through kv and add to hashtable
	int i;
	for (i=0; i<headers_map.size; i++) {
		dpx_header_map* header = (dpx_header_map*) malloc(sizeof(dpx_header_map));

		msgpack_object_raw rawKey = headers[i].key.via.raw;
		char* keyBuf = (char*) malloc(rawKey.size+1);
		strncpy(keyBuf, rawKey.ptr, rawKey.size);
		*(keyBuf+rawKey.size) = '\0';
		header->key = keyBuf;

		msgpack_object_raw rawValue = headers[i].val.via.raw;
		char* valBuf = (char*) malloc(rawValue.size+1);
		strncpy(valBuf, rawValue.ptr, rawValue.size);
		*(valBuf+rawValue.size) = '\0';
		header->value = valBuf;
		
		HASH_ADD_KEYPTR(hh, frame->headers, header->key, strlen(header->key), header);
		headers++;
	}

	msgpack_object_raw errorRaw = o[4].via.raw;
	char* errorBuf = (char*) malloc(errorRaw.size+1);
	strncpy(errorBuf, errorRaw.ptr, errorRaw.size);
	*(errorBuf + errorRaw.size) = '\0';
	frame->error = errorBuf;

	frame->last = o[5].via.i64;

	msgpack_object_raw payloadRaw = o[6].via.raw;
	char* payloadBuf = (char*) malloc(payloadRaw.size);
	strncpy(payloadBuf, payloadRaw.ptr, payloadRaw.size);
	frame->payload = payloadBuf;

	frame->payloadSize = payloadRaw.size;
	
	return frame;
}

msgpack_sbuffer* _dpx_frame_msgpack_to(dpx_frame *frame) {
	msgpack_sbuffer* buf = msgpack_sbuffer_new();
	msgpack_packer* pack = msgpack_packer_new(buf, msgpack_sbuffer_write);

	msgpack_pack_array(pack, DPX_PACK_ARRAY_SIZE);
	msgpack_pack_int(pack, frame->type);
	msgpack_pack_int(pack, frame->channel);

	if (frame->method != NULL) {
		msgpack_pack_raw(pack, strlen(frame->method));
		msgpack_pack_raw_body(pack, frame->method, strlen(frame->method));
	} else {
		msgpack_pack_raw(pack, 0);
		msgpack_pack_raw_body(pack, "", 0);
	}

	msgpack_pack_map(pack, HASH_COUNT(frame->headers));

	dpx_header_map *h;
	for (h=frame->headers; h != NULL; h=h->hh.next) {
		msgpack_pack_raw(pack, strlen(h->key));
		msgpack_pack_raw_body(pack, h->key, strlen(h->key));

		msgpack_pack_raw(pack, strlen(h->value));
		msgpack_pack_raw_body(pack, h->value, strlen(h->value));
	}

	if (frame->error != NULL) {
		msgpack_pack_raw(pack, strlen(frame->error));
		msgpack_pack_raw_body(pack, frame->error, strlen(frame->error));
	} else {
		msgpack_pack_raw(pack, 0);
		msgpack_pack_raw_body(pack, "", 0);
	}

	msgpack_pack_int(pack, frame->last);

	msgpack_pack_raw(pack, frame->payloadSize);
	msgpack_pack_raw_body(pack, frame->payload, frame->payloadSize);

	msgpack_packer_free(pack);
	return buf;
}