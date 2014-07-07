#include <check.h>
#include <pthread.h>
#include <signal.h>
#include <stdlib.h>
#include <sys/types.h>
#include "../dpx-internal.h"

START_TEST(test_dpx_init) {
	dpx_init();
	dpx_cleanup();
} END_TEST

int test_function_ret = 4;

void* test_function(void* v) {
	int j = *(int*)v;
	ck_assert_int_eq(j, 10);
	ck_assert_str_eq(taskgetname(), "dpx_libtask_checker");

	return &test_function_ret;
}

START_TEST(test_dpx_thread_communication) {
	dpx_init();

	_dpx_a a;
	a.function = &test_function;
	int j = 10;
	a.args = &j;

	void* res = _dpx_joinfunc(&a);

	ck_assert_msg(res != NULL, "result is NULL?");
	int cast = *(int*)res;
	ck_assert_int_eq(test_function_ret, cast);

	dpx_cleanup();
} END_TEST

START_TEST(test_dpx_peer_frame_send_receive) {
	dpx_init();

	dpx_peer* p1 = dpx_peer_new();
	dpx_peer* p2 = dpx_peer_new();

	ck_assert_msg(dpx_peer_bind(p1, "127.0.0.1", 9876) == DPX_ERROR_NONE, "Error encountered trying to bind.");
	ck_assert_msg(dpx_peer_connect(p2, "127.0.0.1", 9876) == DPX_ERROR_NONE, "Error encountered trying to connect.");

	dpx_channel* client_chan = dpx_peer_open(p1, "foobar");
	dpx_channel* server_chan = dpx_peer_accept(p2);

	ck_assert_str_eq(dpx_channel_method_get(client_chan), dpx_channel_method_get(server_chan));

	// client -> server

	dpx_frame* client_input = dpx_frame_new(client_chan);

	char* payload = malloc(3);
	*payload = 49;
	*(payload+1) = 50;
	*(payload+2) = 51;

	client_input->payload = payload;
	client_input->payloadSize = 3;

	client_input->last = 1;

	ck_assert_msg(dpx_channel_send_frame(client_chan, client_input) == DPX_ERROR_NONE, "Shouldn't encounter errors sending input to server.");

	dpx_frame* server_input = dpx_channel_receive_frame(server_chan);

	ck_assert_int_eq(client_input->payloadSize, server_input->payloadSize);

	ck_assert_int_eq(*(client_input->payload), *(server_input->payload));
	ck_assert_int_eq(*(client_input->payload + 1), *(server_input->payload + 1));
	ck_assert_int_eq(*(client_input->payload + 2), *(server_input->payload + 2));

	// server -> client

	dpx_frame* server_output = dpx_frame_new(server_chan);

	payload = malloc(3);
	*payload = 51;
	*(payload+1) = 50;
	*(payload+2) = 49;

	server_output->payload = payload;
	server_output->payloadSize = 3;

	server_output->last = 1;

	ck_assert_msg(dpx_channel_send_frame(server_chan, server_output) == DPX_ERROR_NONE, "Shouldn't encounter errors sending input to client.");

	dpx_frame* client_output = dpx_channel_receive_frame(client_chan);

	ck_assert_int_eq(client_output->payloadSize, server_output->payloadSize);

	ck_assert_int_eq(*client_output->payload, *server_output->payload);
	ck_assert_int_eq(*(client_output->payload + 1), *(server_output->payload + 1));
	ck_assert_int_eq(*(client_output->payload + 2), *(server_output->payload + 2));

	free(client_input->payload);
	// FIXME
	//dpx_frame_free(client_input); // aborts because free(client_input) is bad???
	free(server_input->payload);
	//dpx_frame_free(server_input); // ???

	free(server_output->payload);
	//dpx_frame_free(server_output); // ??????
	free(client_output->payload);
	//dpx_frame_free(client_output); // ?????

	// FIXME
	//_dpx_channel_free(client_chan); // ????
	//_dpx_channel_free(server_chan); // ????

	dpx_peer_close(p1);
	dpx_peer_close(p2);

	//dpx_peer_free(p1); // ???
	//dpx_peer_free(p2); // ???

	// FIXME - maybe failing because already being freed due to last? WHAT?

	dpx_cleanup();
} END_TEST

// WARNING: does not free orig
char* test_dpx_reverse(char* orig, int size) {
	char* ret = malloc(size);
	printf("reversing: %.*s\n", 3, orig);
	int i;
	for (i=0; i<size; i++) {
		*(ret + i) = *(orig + size - i - 1);
	}
	printf("reversed: %.*s\n", 3, ret);
	return ret;
}

void test_dpx_call(dpx_peer* peer, char* method, char* payload, int payload_size, char** receive, int* receive_size) {
	dpx_channel* chan = dpx_peer_open(peer, method);
	dpx_frame* req = dpx_frame_new(chan);

	req->payload = payload;
	req->payloadSize = payload_size;
	req->last = 1;

	ck_assert_msg(dpx_channel_send_frame(chan, req) == DPX_ERROR_NONE, "Shouldn't encounter errors sending input.");

	dpx_frame* resp = dpx_channel_receive_frame(chan);
	*receive = resp->payload;
	*receive_size = resp->payloadSize;
	dpx_frame_free(resp);
}

void* test_dpx_receive(void* v) {
	dpx_peer* server = v;

	while (1) {
		dpx_channel* chan = dpx_peer_accept(server);
		printf("method: %s\n", dpx_channel_method_get(chan));
		if (chan != NULL && !strcmp(dpx_channel_method_get(chan), "foo")) {
			dpx_frame* req = dpx_channel_receive_frame(chan);
			dpx_frame* resp = dpx_frame_new(chan);
			resp->payload = test_dpx_reverse(req->payload, req->payloadSize);
			resp->payloadSize = req->payloadSize;

			printf("payload: %.*s, payload size: %d\n", 3, resp->payload, resp->payloadSize);
			ck_assert_msg(dpx_channel_send_frame(chan, resp) == DPX_ERROR_NONE, "failed to send frame back");
		}
	}
}

START_TEST(test_dpx_rpc_call) {
	pid_t pid;

	pid = fork();

	if (pid < 0) {
		ck_assert_msg(0, "failed to fork, which is necessary for this test.");
	}

	if (pid == 0) { // child
		dpx_init();

		dpx_peer* server = dpx_peer_new();
		ck_assert_msg(dpx_peer_bind(server, "127.0.0.1", 9877) == DPX_ERROR_NONE, "Error encountered trying to bind.");

		test_dpx_receive(server);

		dpx_peer_close(server);
		dpx_cleanup();
	} else {
		dpx_init();

		dpx_peer* client = dpx_peer_new();
		ck_assert_msg(dpx_peer_connect(client, "127.0.0.1", 9877) == DPX_ERROR_NONE, "Error encountered trying to connect.");

		char* payload = "123";

		char* receive;
		int receive_size;
		test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);

		if (strncmp(receive, "321", 3)) {
			ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
		}

		kill(pid, SIGKILL); // kill the child, cause I don't care about it anymore.

		dpx_peer_close(client);
		dpx_cleanup();
	}

} END_TEST

Suite*
dpx_suite_core(void)
{
	Suite *s = suite_create("DPX-C Core");

	TCase *tc_core = tcase_create("Basic Functions");
	tcase_add_test(tc_core, test_dpx_init);
	tcase_add_test(tc_core, test_dpx_thread_communication);

	suite_add_tcase(s, tc_core);

	TCase *tc_peer = tcase_create("Peer Functions");
	tcase_add_test(tc_peer, test_dpx_peer_frame_send_receive);
	tcase_add_test(tc_peer, test_dpx_rpc_call);

	suite_add_tcase(s, tc_peer);

	return s;
}

int
main(void)
{
	int number_failed;
	Suite *s = dpx_suite_core();

	SRunner *sr = srunner_create(s);
	srunner_run_all(sr, CK_NORMAL);

	number_failed = srunner_ntests_failed(sr);
	srunner_free(sr);

	return number_failed;
}