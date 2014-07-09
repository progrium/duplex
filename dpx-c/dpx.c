#include "dpx-internal.h"
#include <pthread.h>
#include <sys/socket.h>
#include <sys/un.h>

struct _dpx_context {
	pthread_t task_thread;
	int task_sock;
	char* name;
};

// socket communication
#define DPX_SOCK_LIMIT 1024

// +communication function w/ libtask thread
void* _dpx_joinfunc(dpx_context *c, _dpx_a *a) {
	int fd, len;
	struct sockaddr_un sa;

	if((fd = socket(AF_UNIX, SOCK_STREAM, 0)) < 0){
		return NULL;
	}
	
	memset(&sa, 0, sizeof(sa));
	sa.sun_family = AF_UNIX;
	strcpy(sa.sun_path, c->name);

	len = strlen(sa.sun_path) + sizeof(sa.sun_family);
	while(connect(fd, (struct sockaddr*)&sa, len) == -1){
		fprintf(stderr, "failed to connect, waiting 1 second to try again\n");
		sleep(1);
	}

	if (write(fd, &a, sizeof(void*)) != sizeof(void*)) {
		fprintf(stderr, "failed to write address\n");
		abort();
	}

	void* result = NULL;

	if (read(fd, &result, sizeof(void*)) != sizeof(void*)) {
		fprintf(stderr, "failed to read address\n");
		abort();
	}

	close(fd);

	return result;
}

// +thread libtask
void _dpx_libtask_checker(void* v) {
	dpx_context *c = v;

	lthread_detach();
	DEFINE_LTHREAD;
	//taskname("dpx_libtask_checker_%s", c->name);
	
	if (listen(c->task_sock, DPX_SOCK_LIMIT) == -1) {
		fprintf(stderr, "failed to listen");
		abort();
	}

	int again = 0;

	while(1) {
		int remotesd;

		if ((remotesd = sockaccept(c->task_sock)) == -1) {
			if (!again) {
				fprintf(stderr, "failed to accept, trying again\n");
				again = 1;
				continue;
			} else {
				fprintf(stderr, "failed to accept again, context exiting\n");
				return;
			}
		}

		again = 0;

		_dpx_a *ptr = NULL;

		int res = lthread_read(remotesd, &ptr, sizeof(void*), 0);
		if (res != sizeof(void*)) {
			fprintf(stderr, "failed to handle read\n");
			abort(); // FIXME ?
		}

		void* result = ptr->function(ptr->args);

		if (lthread_write(remotesd, &result, sizeof(void*)) != sizeof(void*)) {
			fprintf(stderr, "failed to write back result\n");
			abort(); // FIXME ?
		}

		shutdown(remotesd, SHUT_WR); // on client side, have to close().
	}
}

// +thread libtask
void* _dpx_libtask_thread(void* v) {
	lthread_t *lt = NULL;

	lthread_create(&lt, &_dpx_libtask_checker, v);
	lthread_run();

	fprintf(stderr, "lthread thread ended unexpectedly");
	abort();
	return NULL;
}

// Initialise the coroutines on a seperate thread.
dpx_context* dpx_init() {
	struct sockaddr_un local;

	pthread_t task_thread;
	int task_sock;
	char* name;

	task_sock = socket(AF_UNIX, SOCK_STREAM, 0);
	if (task_sock == -1) {
		fprintf(stderr, "failed to open socket\n");
		abort();
	}

	char constStr[] = "/tmp/dpxc_XXXXXX";
	name = malloc(strlen(constStr) + 1);
	strcpy(name, constStr);

	int tmpfd;
	if ((tmpfd = mkstemp(name)) == -1) {
		fprintf(stderr, "failed to open a temp file for binding\n");
		abort();
	}

	close(tmpfd);
	unlink(name);

	local.sun_family = AF_UNIX;
	strcpy(local.sun_path, name);

	if (bind(task_sock, (struct sockaddr*)&local, strlen(local.sun_path) + sizeof(local.sun_family)) == -1) {
		fprintf(stderr, "failed to bind\n");
		abort();
	}

	dpx_context *ret = calloc(1, sizeof(dpx_context));
	ret->task_thread = task_thread;
	ret->task_sock = task_sock;
	ret->name = name;

	pthread_create(&task_thread, NULL, &_dpx_libtask_thread, ret);
	pthread_detach(task_thread);

	return ret;
}

void dpx_cleanup(dpx_context *c) {
	unlink(c->name);
	pthread_cancel(c->task_thread);
	close(c->task_sock);

	free(c->name);
	free(c);
}