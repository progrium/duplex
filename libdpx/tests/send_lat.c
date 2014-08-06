#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#include "../dpx.h"

int
main(int argc, char* argv[])
{
	if (argc != 5) {
		fprintf(stderr, "Usage: recv_lat [connect] [port] [msg_size] [num_msgs]\n");
		return 1;
	}

	dpx_init();
	atexit(dpx_cleanup);

	dpx_peer *p = dpx_peer_new();

	if (dpx_peer_connect(p, argv[1], atoi(argv[2])) != DPX_ERROR_NONE) {
		fprintf(stderr, "failed to connect to %s:%d\n", argv[0], atoi(argv[1]));
		return 1;
	}

	int msg_size = atoi(argv[3]);
	int num_msgs = atoi(argv[4]);

	dpx_channel* chan = dpx_peer_open(p, "perf");

	dpx_frame *frame = dpx_frame_new();
	frame->payload = calloc(1, msg_size);
	frame->payloadSize = msg_size;

	struct timespec start;
	struct timespec end;
	clock_gettime(CLOCK_MONOTONIC, &start); // starting time

	int i;
	for (i=0; i<num_msgs; i++) {
		if (dpx_channel_send_frame(chan, frame) != DPX_ERROR_NONE) {
			fprintf(stderr, "failed to send frame to local\n");
			return 1;
		}

		dpx_frame *input = dpx_channel_receive_frame(chan);

		if (input == NULL) {
			fprintf(stderr, "not enough frames received\n");
			return 1;
		}

		if (input->payloadSize != msg_size) {
			fprintf(stderr, "bad size received\n");
			return 1;
		}

		free(input->payload);
		dpx_frame_free(input);
	}

	clock_gettime(CLOCK_MONOTONIC, &end); // ending time

	dpx_channel_free(chan);
	dpx_peer_close(p);

	long int nanoseconds = (end.tv_sec - start.tv_sec) * 1000000000L
						+ end.tv_nsec - start.tv_nsec;

	double latency = (double) nanoseconds / (num_msgs * 2);

	printf("message size: %d bytes\n", msg_size);
	printf("message count: %d\n", num_msgs);
	printf("avg latency: %.3f nanoseconds\n", latency);


	return 0;
}