#include <check.h>
#include <stdlib.h>
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

	printf("1\n");

	dpx_channel* client_chan = dpx_peer_open(p1, "foobar");

		printf("12\n");
	dpx_channel* server_chan = dpx_peer_accept(p2);

		printf("123\n");

	ck_assert_str_eq(dpx_channel_method_get(client_chan), dpx_channel_method_get(server_chan));



	dpx_frame* client_input = dpx_frame_new(client_chan);

	char* payload = malloc(3);
	*payload = 1;
	*(payload+1) = 2;
	*(payload+2) = 3;

	client_input->payload = payload;
	client_input->payloadSize = 3;

	client_input->last = 1;

	ck_assert_msg(dpx_channel_send_frame(client_chan, client_input) == DPX_ERROR_NONE, "Shouldn't encounter errors sending input to server.");

	dpx_frame* server_input = dpx_channel_receive_frame(server_chan);

	ck_assert_int_eq(client_input->payloadSize, server_input->payloadSize);

	ck_assert_int_eq(*client_input->payload, *server_input->payload);
	ck_assert_int_eq(*(client_input->payload + 1), *(server_input->payload + 1));
	ck_assert_int_eq(*(client_input->payload + 2), *(server_input->payload + 2));

	dpx_frame* server_output = dpx_frame_new(server_chan);

	payload = malloc(3);
	*payload = 3;
	*(payload+1) = 2;
	*(payload+2) = 1;

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

	// FIXME
	_dpx_channel_free(client_chan);
	_dpx_channel_free(server_chan);

	dpx_peer_close(p1);
	dpx_peer_close(p2);

	dpx_peer_free(p1);
	dpx_peer_free(p2);

	dpx_cleanup();
} END_TEST

Suite*
dpx_suite_core(void)
{
	Suite *s = suite_create("DPX-C Core");

//	TCase *tc_core = tcase_create("Basic Functions");
//	tcase_add_test(tc_core, test_dpx_init);
//	tcase_add_test(tc_core, test_dpx_thread_communication);

//	suite_add_tcase(s, tc_core);

	TCase *tc_peer = tcase_create("Peer Functions");
	tcase_add_test(tc_peer, test_dpx_peer_frame_send_receive);

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