#include "dpx-internal.h"
#include <errno.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <netdb.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <sys/poll.h>

int
netannounce(int istcp, char *server, int port)
{
	int fd, n, proto;
	struct sockaddr_in sa;
	socklen_t sn;
	uint32_t ip;

	proto = istcp ? SOCK_STREAM : SOCK_DGRAM;
	memset(&sa, 0, sizeof sa);
	sa.sin_family = AF_INET;
	if(server != NULL && strcmp(server, "*") != 0){
		if(netlookup(server, &ip) < 0){
			return -1;
		}
		memmove(&sa.sin_addr, &ip, 4);
	}
	sa.sin_port = htons(port);
	if((fd = lthread_socket(AF_INET, proto, 0)) < 0){
		return -1;
	}
	
	/* set reuse flag for tcp */
	if(istcp && getsockopt(fd, SOL_SOCKET, SO_TYPE, (void*)&n, &sn) >= 0){
		n = 1;
		setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, (char*)&n, sizeof n);
	}

	if(bind(fd, (struct sockaddr*)&sa, sizeof sa) < 0){
		close(fd);
		return -1;
	}

	if(proto == SOCK_STREAM)
		listen(fd, 16);

	return fd;
}

int
netaccept(int fd, char *server, int *port)
{
	int cfd, one;
	struct sockaddr_in sa;
	unsigned char *ip;
	socklen_t len;
	
	len = sizeof sa;
	if((cfd = lthread_accept(fd, (void*)&sa, &len)) < 0){
		return -1;
	}
	if(server){
		ip = (unsigned char*)&sa.sin_addr;
		// FIXME? (was snprint instead of sprintf)
		sprintf(server, "%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3]);
	}
	if(port)
		*port = ntohs(sa.sin_port);
	one = 1;
	setsockopt(cfd, IPPROTO_TCP, TCP_NODELAY, (char*)&one, sizeof one);
	return cfd;
}

int sockaccept(int fd) {
	int cfd;
	struct sockaddr_un sa;
	socklen_t len;
	
	len = sizeof sa;
	if((cfd = lthread_accept(fd, (void*)&sa, &len)) < 0) {
		return -1;
	}

	return cfd;
}

#define CLASS(p) ((*(unsigned char*)(p))>>6)
static int
parseip(char *name, uint32_t *ip)
{
	unsigned char addr[4];
	char *p;
	int i, x;

	p = name;
	for(i=0; i<4 && *p; i++){
		x = strtoul(p, &p, 0);
		if(x < 0 || x >= 256)
			return -1;
		if(*p != '.' && *p != 0)
			return -1;
		if(*p == '.')
			p++;
		addr[i] = x;
	}

	switch(CLASS(addr)){
	case 0:
	case 1:
		if(i == 3){
			addr[3] = addr[2];
			addr[2] = addr[1];
			addr[1] = 0;
		}else if(i == 2){
			addr[3] = addr[1];
			addr[2] = 0;
			addr[1] = 0;
		}else if(i != 4)
			return -1;
		break;
	case 2:
		if(i == 3){
			addr[3] = addr[2];
			addr[2] = 0;
		}else if(i != 4)
			return -1;
		break;
	}
	*ip = *(uint32_t*)addr;
	return 0;
}

int netlookup(char *name, uint32_t *ip) {
	struct hostent *he;

	if(parseip(name, ip) >= 0)
		return 0;
	
	/* BUG - Name resolution blocks.  Need a non-blocking DNS. */
	if((he = gethostbyname(name)) != 0){
		*ip = *(uint32_t*)he->h_addr;
		return 0;
	}
	
	return -1;
}

int netdial(int istcp, char *server, int port) {
	int proto, fd, n;
	uint32_t ip;
	struct sockaddr_in sa;
	socklen_t sn;
	
	if(netlookup(server, &ip) < 0)
		return -1;

	proto = istcp ? SOCK_STREAM : SOCK_DGRAM;
	if((fd = lthread_socket(AF_INET, proto, 0)) < 0){
		return -1;
	}

	/* for udp */
	if(!istcp){
		n = 1;
		setsockopt(fd, SOL_SOCKET, SO_BROADCAST, &n, sizeof n);
	}
	
	/* start connecting */
	memset(&sa, 0, sizeof sa);
	memmove(&sa.sin_addr, &ip, 4);
	sa.sin_family = AF_INET;
	sa.sin_port = htons(port);
	if(lthread_connect(fd, (struct sockaddr*)&sa, sizeof sa, 0) < 0 && errno != EINPROGRESS){
		close(fd);
		return -1;
	}

	/* wait for finish */	
	sn = sizeof sa;
	if(getpeername(fd, (struct sockaddr*)&sa, &sn) >= 0){
		return fd;
	}
	
	/* report error */
	sn = sizeof n;
	getsockopt(fd, SOL_SOCKET, SO_ERROR, (void*)&n, &sn);
	if(n == 0)
		n = ECONNREFUSED;
	close(fd);
	errno = n;
	return -1;
}