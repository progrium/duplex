#include "dpx-internal.h"

void dpx_frame_free(dpx_frame *frame) {
	if (frame->errCh != NULL) {
		alchanclose(frame->errCh);
		// empty channel
		DPX_ERROR err;
		while (alchanrecv(frame->errCh, &err) != ALCHAN_CLOSED) {}

		alchanfree(frame->errCh);
	}

	if (frame->method != NULL)
		free(frame->method);
	if (frame->error != NULL)
		free(frame->error);

	dpx_header_map *current, *tmp;
	HASH_ITER(hh, frame->headers, current, tmp) {
		HASH_DEL(frame->headers, current);
		free(current->key);
		free(current->value);
		free(current);
	}
	free(frame);
}

void dpx_frame_init(dpx_frame *frame) {
	frame->errCh = NULL;
	frame->chanRef = NULL;

	frame->type = 0;
	frame->channel = DPX_FRAME_NOCH;
	frame->method = NULL;
	
	frame->headers = NULL;

	frame->error = NULL;
	frame->last = 0;

	frame->payload = NULL;
	frame->payloadSize = 0;
}

dpx_frame* dpx_frame_new() {
	dpx_frame *frame = malloc(sizeof(dpx_frame));

	dpx_frame_init(frame);

	return frame;
}

void _dpx_frame_copy_header_iter(void* arg, char* k, char* v) {
	dpx_frame *new = arg;

	dpx_frame_header_add(new, k, v);
}

void dpx_frame_copy(dpx_frame *new, const dpx_frame *old) {
	memset(new, 0, sizeof(dpx_frame));

	new->chanRef = old->chanRef;
	new->type = old->type;

	new->channel = old->channel;

	if (old->method != NULL) {
		new->method = malloc(strlen(old->method) + 1);
		strcpy(new->method, old->method);
	}

	dpx_frame_header_iter(old, _dpx_frame_copy_header_iter, new);

	if (old->error != NULL) {
		new->error = malloc(strlen(old->error) + 1);
		strcpy(new->error, old->error);
	}

	new->last = old->last;

	new->payload = malloc(old->payloadSize);
	memmove(new->payload, old->payload, old->payloadSize);
	
	new->payloadSize = old->payloadSize;
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

void dpx_frame_header_iter(dpx_frame *frame, void (*iter_func)(void* arg, char* k, char* v), void* arg) {
	dpx_header_map *cur, *next;

	HASH_ITER(hh, frame->headers, cur, next) {
		iter_func(arg, cur->key, cur->value);
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
	DEBUG_FUNC(
		msgpack_object_print(stdout, *obj);
		puts("");
	);
	msgpack_object_array arr = obj->via.array;

	assert(arr.size == DPX_PACK_ARRAY_SIZE);

	// create frame
	dpx_frame* frame = dpx_frame_new();

	msgpack_object* o = arr.ptr;

	frame->type = o[0].via.i64;

	frame->channel = o[1].via.i64;

	msgpack_object methodObj = o[2];
	if (methodObj.type == MSGPACK_OBJECT_NIL) {
		frame->method = NULL;
	} else {
		msgpack_object_raw methodRaw = methodObj.via.raw;
		char* methodBuf = (char*) malloc(methodRaw.size+1);
		strncpy(methodBuf, methodRaw.ptr, methodRaw.size);
		*(methodBuf + methodRaw.size) = '\0';
		frame->method = methodBuf;
	}

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

	msgpack_object errorObj = o[4];

	if (errorObj.type == MSGPACK_OBJECT_NIL) {
		frame->error = NULL;
	} else {
		msgpack_object_raw errorRaw = errorObj.via.raw;
		char* errorBuf = (char*) malloc(errorRaw.size+1);
		strncpy(errorBuf, errorRaw.ptr, errorRaw.size);
		*(errorBuf + errorRaw.size) = '\0';
		frame->error = errorBuf;
	}

	if (o[5].via.boolean)
		frame->last = 1;
	else
		frame->last = 0;

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
		msgpack_pack_nil(pack);
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
		msgpack_pack_nil(pack);
	}

	if (frame->last)
		msgpack_pack_true(pack);
	else
		msgpack_pack_false(pack);

	msgpack_pack_raw(pack, frame->payloadSize);
	msgpack_pack_raw_body(pack, frame->payload, frame->payloadSize);

	msgpack_packer_free(pack);
	return buf;
}