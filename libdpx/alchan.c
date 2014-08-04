#include "dpx-internal.h"
#include <string.h>


struct _al_ch_list;
typedef struct _al_ch_list _al_ch_list;

struct _al_ch_list {
	_al_ch_list		*prev;
	void*			elem;
	_al_ch_list		*next;
};

struct al_channel
{
    Rendez          *rcond;
    Rendez          *wcond;

	int				closed;

	size_t			elemsize;

	unsigned int	bufsize;
	unsigned int	cursize;

	_al_ch_list		*head;
	_al_ch_list		*tail;
};

al_channel*
alchancreate(size_t elemsize, unsigned int buffersize)
{
	al_channel* chan = calloc(1, sizeof(al_channel));
    chan->rcond = calloc(1, sizeof(Rendez));
    chan->wcond = calloc(1, sizeof(Rendez));

	chan->elemsize = elemsize;
	chan->bufsize = buffersize;

	return chan;
}

void
alchanclose(al_channel *c)
{
	c->closed = 1;
    taskwakeupall(c->rcond);
    taskwakeupall(c->wcond);
	// ^ anything waiting on a channel must now realise it's closed
}

int
alchanfree(al_channel *c)
{
	if (!c->closed)
		return 0;
	if (c->head != NULL)
		return 0;
	free(c->rcond);
	free(c->wcond);
	free(c);

	return 1;
}

int
alchannbrecv(al_channel *c, void *v)
{
	if (c->head == NULL) {
		if (c->closed)
			return ALCHAN_CLOSED;
		else
			return ALCHAN_NONE;
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

	// signal to wcond that we just freed an element
	taskwakeupall(c->wcond);

	return 0;
}

void*
alchannbrecvp(al_channel *c)
{
	void* pointer = NULL;
	alchannbrecv(c, &pointer);
	return pointer;
}

unsigned long
alchannbrecvul(al_channel *c)
{
	unsigned long value;
	alchannbrecv(c, &value);
	return value;
}

int
alchanrecv(al_channel *c, void *v)
{
	while (1)
	{
		int result = alchannbrecv(c, v);
		if (result != ALCHAN_NONE)
			return result;

		tasksleep(c->rcond);
	}
}

void*
alchanrecvp(al_channel *c)
{
	void* pointer = NULL;
	alchanrecv(c, &pointer);
	return pointer;
}

unsigned long
alchanrecvul(al_channel *c)
{
	unsigned long value;
	alchannbrecv(c, &value);
	return value;
}

int
_alchansend(al_channel *c, void *v, int block)
{
	if (c->closed)
		return ALCHAN_CLOSED;

	// special condition for 0
	// if the buffer size is 0 (synchronous), then we will go over the 
	// the buffer limit regardless... (and block later until chan receieves it)

	while (1)
	{
		int cond = (c->cursize >= c->bufsize);
		if (c->bufsize == 0)
			cond = (c->cursize > c->bufsize);

		if (!cond) {
			break;
		}

		if (block) {
			tasksleep(c->wcond);
		} else {
			return ALCHAN_FULL;
		}
	}

	// check again in case we close the channel
	if (c->closed)
		return ALCHAN_CLOSED;

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

	// let readers know that we've just inserted an element
	taskwakeupall(c->rcond);

	// if buffer is overfull and we're blocking, block
	// (special condition for 0: block always)
	if (block || c->bufsize == 0)
	{
		while (c->cursize > c->bufsize)
		{
			tasksleep(c->wcond);
		}
	}

	return 0;
}

int
alchannbsend(al_channel *c, void *v)
{
	return _alchansend(c, v, 0);
}

int
alchannbsendp(al_channel *c, void *v)
{
	return alchannbsendp(c, &v);
}

int
alchannbsendul(al_channel *c, unsigned long v)
{
	return alchannbsend(c, &v);
}

int
alchansend(al_channel *c, void *v)
{
	return _alchansend(c, v, 1);
}

int
alchansendp(al_channel *c, void *v)
{
	return alchansendp(c, &v);
}

int
alchansendul(al_channel *c, unsigned long v)
{
	return alchansend(c, &v);
}
