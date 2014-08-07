NAME=libtask

SHARED=$(NAME).so
STATIC=$(NAME).a

DESTDIR=
PREFIX=/usr/local

TCPLIBS=

ASM=asm.o
OFILES=\
	$(ASM)\
	channel.o\
	context.o\
	fd.o\
	net.o\
	print.o\
	qlock.o\
	rendez.o\
	task.o\

all: $(STATIC) $(SHARED)

$(OFILES): taskimpl.h task.h 386-ucontext.h power-ucontext.h

AS=gcc -c
CC=gcc
CFLAGS=-Wall -I. -ggdb

%.o: %.S
	$(AS) $*.S

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c

$(SHARED): $(OFILES)
	$(CC) $(CFLAGS) -shared -o $(SHARED) $(OFILES)

$(STATIC): $(OFILES)
	ar rvc $(STATIC) $(OFILES)

primes: primes.o $(STATIC)
	$(CC) -o primes primes.o $(STATIC)

tcpproxy: tcpproxy.o $(STATIC)
	$(CC) -o tcpproxy tcpproxy.o $(STATIC) $(TCPLIBS)

httpload: httpload.o $(STATIC)
	$(CC) -o httpload httpload.o $(STATIC)

testdelay: testdelay.o $(STATIC)
	$(CC) -o testdelay testdelay.o $(STATIC)

testdelay1: testdelay1.o $(STATIC)
	$(CC) -o testdelay1 testdelay1.o $(STATIC)

clean:
	rm -f *.o primes tcpproxy testdelay testdelay1 httpload $(STATIC) $(SHARED)

install: $(STATIC) $(SHARED)
	cp $(SHARED) $(DESTDIR)$(PREFIX)/lib
	cp $(STATIC) $(DESTDIR)$(PREFIX)/lib
	cp *.h $(DESTDIR)$(PREFIX)/include

