#include "dpx-c.h"

void dpx_frame_free(dpx_frame *frame) {
	chanfree(frame->errCh);
	free(frame);
}

dpx_frame* dpx_frame_new(dpx_channel *ch) {
	dpx_frame *frame = (dpx_frame*) malloc(sizeof(dpx_frame));

	frame->_struct = 0;
	frame->errCh = chancreate(sizeof(DPX_ERROR), 0);
	frame->chanRef = ch;

	frame->type = 0;
	frame->channel = ch->id;

	frame->method = NULL;
	frame->headers = NULL;
	frame->error = NULL;
	frame->last = 0;

	frame->payload = NULL;

	return frame;
}