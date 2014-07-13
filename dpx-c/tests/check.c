#include <check.h>
#include <pthread.h>
#include <signal.h>
#include <stdlib.h>
#include <sys/types.h>
#include "../dpx-internal.h"

START_TEST(test_dpx_init) {
	dpx_context *c = dpx_init();
	dpx_cleanup(c);
} END_TEST

int test_function_ret = 4;

void* test_function(void* v) {
	int j = *(int*)v;
	ck_assert_int_eq(j, 10);
	//ck_assert_str_eq(taskgetname(), "dpx_libtask_checker");

	return &test_function_ret;
}

START_TEST(test_dpx_thread_communication) {
	dpx_context *c = dpx_init();

	_dpx_a a;
	a.function = &test_function;
	int j = 10;
	a.args = &j;

	void* res = _dpx_joinfunc(c, &a);

	ck_assert_msg(res != NULL, "result is NULL?");
	int cast = *(int*)res;
	ck_assert_int_eq(test_function_ret, cast);

	dpx_cleanup(c);
} END_TEST

START_TEST(test_dpx_peer_frame_send_receive) {
	dpx_context *c = dpx_init();

	dpx_peer* p1 = dpx_peer_new(c);
	dpx_peer* p2 = dpx_peer_new(c);

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
	dpx_frame_free(client_input);
	free(server_input->payload);
	dpx_frame_free(server_input);

	free(server_output->payload);
	dpx_frame_free(server_output);
	free(client_output->payload);
	dpx_frame_free(client_output);

	dpx_channel_free(client_chan);
	dpx_channel_free(server_chan);

	dpx_peer_close(p1);
	dpx_peer_close(p2);

	dpx_peer_free(p1);
	dpx_peer_free(p2);

	dpx_cleanup(c);
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
		if (chan == NULL)
			pthread_exit(NULL);

		if (!strcmp(dpx_channel_method_get(chan), "foo")) {
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

	dpx_context *server_context = dpx_init();

	dpx_peer* server = dpx_peer_new(server_context);
	ck_assert_msg(dpx_peer_bind(server, "127.0.0.1", 9877) == DPX_ERROR_NONE, "Error encountered trying to bind.");

	pthread_t server_thread;

	pthread_create(&server_thread, NULL, &test_dpx_receive, server);

	dpx_context *client_context = dpx_init();

	dpx_peer* client = dpx_peer_new(client_context);
	ck_assert_msg(dpx_peer_connect(client, "127.0.0.1", 9877) == DPX_ERROR_NONE, "Error encountered trying to connect.");

	char* payload = "123";

	char* receive;
	int receive_size;
	test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);

	if (strncmp(receive, "321", 3)) {
		ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
	}

	dpx_peer_close(server);

	pthread_join(server_thread, NULL);

	dpx_peer_close(client);
	dpx_cleanup(client_context);

	dpx_cleanup(server_context);

} END_TEST

struct _test_dri {
	dpx_peer* server;
	char id;
};

void* test_dpx_receive_id(void* v) {
	struct _test_dri *t = v;
	dpx_peer* server = t->server;
	char addto = t->id;

	while (1) {
		dpx_channel* chan = dpx_peer_accept(server);
		if (chan == NULL)
			pthread_exit(NULL);

		if (!strcmp(dpx_channel_method_get(chan), "foo")) {
			dpx_frame* req = dpx_channel_receive_frame(chan);
			dpx_frame* resp = dpx_frame_new(chan);
			resp->payload = test_dpx_reverse(req->payload, req->payloadSize);
			resp->payloadSize = req->payloadSize + 1;

			resp->payload = realloc(resp->payload, resp->payloadSize);
			*(resp->payload + 3) = addto;

			printf("payload: %.*s, payload size: %d\n", 4, resp->payload, resp->payloadSize);
			ck_assert_msg(dpx_channel_send_frame(chan, resp) == DPX_ERROR_NONE, "failed to send frame back");
		}
	}
}

START_TEST(test_dpx_round_robin_async) {

	dpx_context *client_context = dpx_init();
	dpx_peer *client = dpx_peer_new(client_context);

	ck_assert_msg(dpx_peer_connect(client, "127.0.0.1", 9876) == DPX_ERROR_NONE, "client failed to queue connect to 9876");
	ck_assert_msg(dpx_peer_connect(client, "127.0.0.1", 9875) == DPX_ERROR_NONE, "client failed to queue connect to 9875");
	ck_assert_msg(dpx_peer_bind(client, "127.0.0.1", 9874) == DPX_ERROR_NONE, "client failed to queue bind to 9874");

	dpx_context *server1context = dpx_init();
	dpx_peer* server1 = dpx_peer_new(server1context);
	ck_assert_msg(dpx_peer_bind(server1, "127.0.0.1", 9876) == DPX_ERROR_NONE, "server1 failed to queue bind to 9876");

	struct _test_dri *t1 = malloc(sizeof(struct _test_dri));
	t1->server = server1;
	t1->id = '1';

	pthread_t server1thread;
	pthread_create(&server1thread, NULL, &test_dpx_receive_id, t1);

	dpx_context *server2context = dpx_init();
	dpx_peer* server2 = dpx_peer_new(server2context);
	ck_assert_msg(dpx_peer_bind(server2, "127.0.0.1", 9875) == DPX_ERROR_NONE, "server2 failed to queue bind to 9875");

	struct _test_dri *t2 = malloc(sizeof(struct _test_dri));
	t2->server = server2;
	t2->id = '2';

	pthread_t server2thread;
	pthread_create(&server2thread, NULL, &test_dpx_receive_id, t2);

	dpx_context *server3context = dpx_init();
	dpx_peer* server3 = dpx_peer_new(server3context);
	ck_assert_msg(dpx_peer_connect(server3, "127.0.0.1", 9874) == DPX_ERROR_NONE, "server3 failed to queue connect to 9874");

	struct _test_dri *t3 = malloc(sizeof(struct _test_dri));
	t3->server = server3;
	t3->id = '3';

	pthread_t server3thread;
	pthread_create(&server3thread, NULL, &test_dpx_receive_id, t3);

	char* payload = "123";

	char* receive;
	int receive_size;

	char servers_checked[4];

	// first server
	test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);
	if (strncmp(receive, "321", 3)) {
		ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
	}
	servers_checked[0] = *(receive + 3);
	free(receive);

	// second
	test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);
	if (strncmp(receive, "321", 3)) {
		ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
	}
	servers_checked[1] = *(receive + 3);
	free(receive);

	// third
	test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);
	if (strncmp(receive, "321", 3)) {
		ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
	}
	servers_checked[2] = *(receive + 3);
	free(receive);

	// first again
	test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);
	if (strncmp(receive, "321", 3)) {
		ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
	}
	servers_checked[3] = *(receive + 3);
	free(receive);

	if (memchr(servers_checked, '1', 4) == NULL)
		ck_abort_msg("failed to visit server 1");
	if (memchr(servers_checked, '2', 4) == NULL)
		ck_abort_msg("failed to visit server 2");
	if (memchr(servers_checked, '3', 4) == NULL)
		ck_abort_msg("failed to visit server 3");

	dpx_peer_close(server1);
	dpx_peer_close(server2);
	dpx_peer_close(server3);

	pthread_join(server1thread, NULL);
	pthread_join(server2thread, NULL);
	pthread_join(server3thread, NULL);

	dpx_peer_close(client);
	dpx_cleanup(client_context);
	dpx_cleanup(server1context);
	dpx_cleanup(server2context);
	dpx_cleanup(server3context);

} END_TEST

START_TEST(test_dpx_async_messaging) {

	dpx_context *client_context = dpx_init();

	dpx_peer* client = dpx_peer_new(client_context);
	ck_assert_msg(dpx_peer_connect(client, "127.0.0.1", 9877) == DPX_ERROR_NONE, "Error encountered trying to connect.");

	dpx_context *server_context = dpx_init();

	dpx_peer* server = dpx_peer_new(server_context);
	ck_assert_msg(dpx_peer_bind(server, "127.0.0.1", 9877) == DPX_ERROR_NONE, "Error encountered trying to bind.");

	pthread_t server_thread;

	pthread_create(&server_thread, NULL, &test_dpx_receive, server);
	pthread_detach(server_thread);

	char* payload = "123";

	char* receive;
	int receive_size;
	test_dpx_call(client, "foo", payload, 3, &receive, &receive_size);

	if (strncmp(receive, "321", 3)) {
		ck_assert_msg(0, "Got bad response from strncmp: %.*s", 3, receive);
	}

	pthread_cancel(server_thread);

	dpx_peer_close(client);
	dpx_cleanup(client_context);

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

	TCase *tc_async = tcase_create("Async Functions");
	tcase_add_test(tc_async, test_dpx_round_robin_async);
	tcase_add_test(tc_async, test_dpx_async_messaging);
	tcase_set_timeout(tc_async, 10);

	suite_add_tcase(s, tc_async);

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
