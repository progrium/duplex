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

START_TEST(test_dpx_peer_creation) {
	dpx_init();

	dpx_peer* p1 = dpx_peer_new();
	dpx_peer* p2 = dpx_peer_new();

	dpx_peer_free(p1);
	dpx_peer_free(p2);

	dpx_cleanup();
} END_TEST

Suite*
dpx_suite_core(void)
{
	Suite *s = suite_create("DPX-C Core");

	TCase *tc_core = tcase_create("Basic Functions");
	tcase_add_test(tc_core, test_dpx_init);
	tcase_add_test(tc_core, test_dpx_thread_communication);
	tcase_add_test(tc_core, test_dpx_peer_creation);

	suite_add_tcase(s, tc_core);

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