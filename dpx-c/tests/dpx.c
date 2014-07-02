#include <check.h>
#include <stdlib.h>
#include "../dpx-internal.h"

START_TEST(test_dpx_init) {
	dpx_init();
} END_TEST

Suite*
dpx_suite(void)
{
	Suite *s = suite_create("DPX Threadsafe");

	TCase *tc_core = tcase_create("Core");
	tcase_add_test(tc_core, test_dpx_init);
	suite_add_tcase(s, tc_core);

	return s;
}

int
main(void)
{
	int number_failed;
	Suite *s = dpx_suite();

	SRunner *sr = srunner_create(s);
	srunner_run_all(sr, CK_NORMAL);

	number_failed = srunner_ntests_failed(sr);
	srunner_free(sr);

	return number_failed;
}