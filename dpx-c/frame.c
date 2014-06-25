#include "dpx-c.h"

void dpx_frame_free(dpx_frame *frame) {
	chanfree(frame->errCh);
	free(frame->method);
	free(frame->error);
	HASH_CLEAR(hh, frame->headers);
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