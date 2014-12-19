#include "dpx-internal.h"
#include <string.h>


struct _al_ch_list;
typedef struct _al_ch_list _al_ch_list;

struct _al_ch_list {
	_al_ch_list		*prev;
	void*			elem;
	_al_ch_list		*next;
};

struct al_channel {
	QLock			lock;

	Rendez          wcond;

	int				closed;

	size_t			elemsize;

	unsigned int	bufsize;
	unsigned int	cursize;

	_al_ch_list		*head;
	_al_ch_list		*tail;
};

al_channel*
alchancreate(size_t elemsize, unsigned int buffersize) {
	al_channel* chan = calloc(1, sizeof(al_channel));

	chan->elemsize = elemsize;
	chan->bufsize = buffersize;

	return chan;
}

void
alchanclose(al_channel *c) {
	qlock(&c->lock);

	c->closed = 1;

	taskwakeupall(&c->wcond);
	// ^ anything waiting on a channel must now realise it's closed

	qunlock(&c->lock);
}

int
alchanfree(al_channel *c) {
	if (!c->closed)
		return 0;
	if (c->head != NULL)
		return 0;
	free(c);

	return 1;
}

int
alchannbrecv(al_channel *c, void *v) {
	int ret = 0;

	qlock(&c->lock);

	if (c->head == NULL) {
		if (c->closed)
			ret = ALCHAN_CLOSED;
		else
			ret = ALCHAN_NONE;

		goto _alchannbrecv_cleanup;
	}

	_al_ch_list *head = c->head;

	memmove(v, head->elem, c->elemsize);

	if (c->head == c->tail) { // one element left
		c->head = NULL;
		c->tail = NULL;
	} else {
		c->head = head->next;
		c->head->prev = NULL;
	}

	c->cursize--;
	free(head->elem);
	free(head);

_alchannbrecv_cleanup:
	qunlock(&c->lock);

	// signal regardless
	taskwakeupall(&c->wcond);

	return ret;
}

void*
alchannbrecvp(al_channel *c) {
	void* pointer = NULL;
	alchannbrecv(c, &pointer);
	return pointer;
}

unsigned long
alchannbrecvul(al_channel *c) {
	unsigned long *value;
	if (alchannbrecv(c, &value))
		return 0;
	unsigned long ret = *value;
	free(value);
	return ret;
}

int
alchanrecv(al_channel *c, void *v) {
	while (1) {
		int result = alchannbrecv(c, v);
		if (result != ALCHAN_NONE)
			return result;

		taskdelay(0);
	}
}

void*
alchanrecvp(al_channel *c) {
	void* pointer = NULL;
	alchanrecv(c, &pointer);
	return pointer;
}

unsigned long
alchanrecvul(al_channel *c) {
	unsigned long *value;
	if (alchanrecv(c, &value))
		return 0;
	unsigned long ret = *value;
	free(value);
	return ret;
}

int
_alchansend(al_channel *c, void *v, int block) {
	// special condition for 0
	// if the buffer size is 0 (synchronous), then we will go over the
	// the buffer limit regardless... (and block later until chan receieves it)

	int ret = 0;

	qlock(&c->lock);

	while (1) {
		if (c->closed) {
			ret = ALCHAN_CLOSED;
			goto _alchansend_cleanup;
		}

		int cond = (c->cursize >= c->bufsize);
		if (c->bufsize == 0)
			cond = (c->cursize > c->bufsize);

		if (!cond) {
			break;
		}

		if (block) {
			qunlock(&c->lock);
			tasksleep(&c->wcond);
			qlock(&c->lock);
		} else {
			ret = ALCHAN_FULL;
			goto _alchansend_cleanup;
		}
	}

	_al_ch_list *elem = malloc(sizeof(_al_ch_list));
	elem->prev = c->tail;
	elem->elem = malloc(c->elemsize);
	memmove(elem->elem, v, c->elemsize);
	elem->next = NULL;

	if (c->head == NULL) { // no elements
		c->head = elem;
		c->tail = elem;
	} else {
		c->tail->next = elem;
		c->tail = elem;
	}

	c->cursize++;

	// if buffer is overfull and we're blocking, block
	// (special condition for 0: block always)
	if (block || c->bufsize == 0) {
		while (c->cursize > c->bufsize) {
			qunlock(&c->lock);
			tasksleep(&c->wcond);
			qlock(&c->lock);
		}
	}

_alchansend_cleanup:
	qunlock(&c->lock);

	return ret;
}

int
alchannbsend(al_channel *c, void *v) {
	return _alchansend(c, v, 0);
}

int
alchannbsendp(al_channel *c, void *v) {
	return alchannbsend(c, &v);
}

int
alchannbsendul(al_channel *c, unsigned long v) {
	unsigned long *ptr = malloc(sizeof(unsigned long));
	*ptr = v;
	return alchannbsend(c, &ptr);
}

int
alchansend(al_channel *c, void *v) {
	return _alchansend(c, v, 1);
}

int
alchansendp(al_channel *c, void *v) {
	return alchansend(c, &v);
}

int
alchansendul(al_channel *c, unsigned long v) {
	unsigned long *ptr = malloc(sizeof(unsigned long));
	*ptr = v;
	return alchansend(c, &ptr);
}
