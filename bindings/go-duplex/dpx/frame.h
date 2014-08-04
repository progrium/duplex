#include <dpx.h>
#include <stdlib.h>
#include <string.h>

void
convert_payload(dpx_frame *f, void* payload, int payload_size);

void
header_helper(void *f, char* key, char* val);