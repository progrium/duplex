#include "frame.h"

void
convert_payload(dpx_frame *f, void* payload, int payload_size)
{
	f->payload = malloc(payload_size);
	f->payloadSize = payload_size;
	memmove(f->payload, payload, payload_size);
}

void
header_helper(void *f, char* key, char* val) {
	helperAdd(f, key, val);
}