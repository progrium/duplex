#include "dpx-internal.h"
#include <pthread.h>
#include <sys/socket.h>
#include <sys/un.h>

pthread_t task_thread;

// socket communication
#define DPX_SOCK_LIMIT 1024

int task_sock;
struct sockaddr_un local;

char* name;

// +communication function w/ libtask thread
void* _dpx_joinfunc(_dpx_a *a) {
	int fd, len;
	struct sockaddr_un sa;

	if((fd = socket(AF_UNIX, SOCK_STREAM, 0)) < 0){
		return NULL;
	}
	
	memset(&sa, 0, sizeof(sa));
	sa.sun_family = AF_UNIX;
	strcpy(sa.sun_path, name);

	len = strlen(sa.sun_path) + sizeof(sa.sun_family);
	if(connect(fd, (struct sockaddr*)&sa, len) == -1){
		fprintf(stderr, "failed to connect\n");
		abort();
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
	if (listen(task_sock, DPX_SOCK_LIMIT) == -1) {
		fprintf(stderr, "failed to listen");
		return;
	}

	while(1) {
		int remotesd;

		if ((remotesd = sockaccept(task_sock)) == -1) {
			fprintf(stderr, "failed to accept");
			close(remotesd);
			return;
		}

		_dpx_a *ptr = NULL;

		int res = fdread1(remotesd, &ptr, sizeof(void*));
		if (res != sizeof(void*)) {
			fprintf(stderr, "failed to handle read\n");
			abort(); // FIXME ?
		}

		void* result = ptr->function(ptr->args);

		if (fdwrite(remotesd, &result, sizeof(void*)) != sizeof(void*)) {
			fprintf(stderr, "failed to write back result\n");
			abort(); // FIXME ?
		}

		shutdown(remotesd, SHUT_WR); // on client side, have to close().
	}
}

// +thread libtask
void* _dpx_libtask_thread(void* v) {
	taskinit(&_dpx_libtask_checker, NULL);

	fprintf(stderr, "libtask thread ended unexpectedly");
	abort();
	return NULL;
}

// Initialise the coroutines on a seperate thread.
void dpx_init() {
	task_sock = socket(AF_UNIX, SOCK_STREAM, 0);
	if (task_sock == -1) {
		fprintf(stderr, "failed to open socket\n");
		abort();
	}

	char constStr[] = "/tmp/dpxc_XXXXXX";
	name = malloc(strlen(constStr));
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

	pthread_create(&task_thread, NULL, &_dpx_libtask_thread, NULL);
	pthread_detach(task_thread);
}