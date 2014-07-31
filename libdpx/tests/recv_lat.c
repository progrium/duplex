#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#include "../dpx.h"

int
main(int argc, char* argv[])
{
	if (argc != 5) {
		fprintf(stderr, "Usage: recv_lat [bind] [port] [msg_size] [num_msgs]\n");
		return 1;
	}

	dpx_init();
	atexit(dpx_cleanup);

	dpx_peer *p = dpx_peer_new();

	if (dpx_peer_bind(p, argv[1], atoi(argv[2])) != DPX_ERROR_NONE) {
		fprintf(stderr, "failed to bind to %s:%d\n", argv[0], atoi(argv[1]));
		return 1;
	}

	int msg_size = atoi(argv[3]);
	int num_msgs = atoi(argv[4]);

	dpx_channel* chan = dpx_peer_accept(p);

	int i;
	for (i=0; i<num_msgs; i++) {
		dpx_frame *input = dpx_channel_receive_frame(chan);

		if (input == NULL) {
			fprintf(stderr, "not enough frames received\n");
			return 1;
		}

		if (input->payloadSize != msg_size) {
			fprintf(stderr, "bad size received\n");
			return 1;
		}
		
		if (dpx_channel_send_frame(chan, input) != DPX_ERROR_NONE) {
			fprintf(stderr, "failed to send frame back to remote\n");
			return 1;
		}
	}

	dpx_channel_free(chan);
	dpx_peer_close(p);

	return 0;
}