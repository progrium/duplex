#include "dpx-internal.h"
#include <errno.h>
#include <fcntl.h>
#include <pthread.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <time.h>
#include <unistd.h>

struct _dpx_context {
	pthread_t task_thread;
	int task_sock;
	char* name;
};

// socket communication
#define DPX_SOCK_LIMIT 512

dpx_context* c = NULL;

// +communication function w/ libtask thread
void* _dpx_joinfunc(_dpx_a *a) {
	int fd, len;
	struct sockaddr_un sa;

	if((fd = socket(AF_UNIX, SOCK_STREAM, 0)) < 0) {
		return NULL;
	}

	memset(&sa, 0, sizeof(sa));
	sa.sun_family = AF_UNIX;
	strcpy(sa.sun_path, c->name);

	len = strlen(sa.sun_path) + sizeof(sa.sun_family);
	while(connect(fd, (struct sockaddr*)&sa, len) == -1) {
		struct timespec t;
		t.tv_sec = 0;
		t.tv_nsec = 5000;
		nanosleep(&t, NULL);
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
void _dpx_libtask_handler(void* v) {
	taskname("_dpx_libtask_handler");

	int* store = v;
	int remotesd = *store;
	free(store);

	_dpx_a *ptr = NULL;

	fdnoblock(remotesd);

	int res = fdread(remotesd, &ptr, sizeof(void*));
	if (res != sizeof(void*)) {
		fprintf(stderr, "failed to handle read\n");
		abort(); // FIXME ?
	}

	void* result = ptr->function(ptr->args);

	if (fdwrite(remotesd, &result, sizeof(void*)) != sizeof(void*)) {
		fprintf(stderr, "failed to write back result\n");
		abort(); // FIXME ?
	}

	close(remotesd);
}

// +thread libtask
void _dpx_libtask_checker(void* v) {
	taskname("dpx_libtask_checker");

	if (listen(c->task_sock, DPX_SOCK_LIMIT) == -1) {
		fprintf(stderr, "failed to listen");
		abort();
	}

	while(1) {
		int remotesd;

		if ((remotesd = sockaccept(c->task_sock)) == -1) {
			perror("failed to accept request, yielding: ");
			taskdelay(0); // because taskyield does NOTHING
			continue;
		}

		int* store = malloc(sizeof(int));
		*store = remotesd;

		taskcreate(&_dpx_libtask_handler, store, DPX_TASK_STACK_SIZE);
	}
}

// +thread libtask
void* _dpx_libtask_thread(void* v) {
	taskinit(&_dpx_libtask_checker, v);

	fprintf(stderr, "libtask thread ended unexpectedly");
	abort();
	return NULL;
}

// Initialise the coroutines on a seperate thread.
void dpx_init() {
	if (c != NULL) {
		fprintf(stderr, "Existing global context exists!\n");
		abort();
	}

	struct sockaddr_un local;

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

	int flags = fcntl(task_sock, F_GETFL, 0);
	if (fcntl(task_sock, F_SETFL, flags | O_NONBLOCK) == -1) {
		fprintf(stderr, "faied to open task socket");
		abort();
	}

	DEBUG_FUNC(printf("dpx_init: sock @ %s\n", name));

	c = calloc(1, sizeof(dpx_context));
	c->task_sock = task_sock;
	c->name = name;

	int err = pthread_create(&c->task_thread, NULL, &_dpx_libtask_thread, NULL);

	if (err) {
		fprintf(stderr, "failed to initialise dpx_init thread: %s\n", strerror(err));
		abort();
	}
	pthread_detach(c->task_thread);
}

void dpx_cleanup() {
	if (c == NULL) {
		fprintf(stderr, "Context not set!\n");
		abort();
	}

	DEBUG_FUNC(printf("dpx_cleanup: sock @ %s\n", c->name));
	unlink(c->name);
	pthread_cancel(c->task_thread);
	close(c->task_sock);

	free(c->name);
	free(c);
	c = NULL;
}
