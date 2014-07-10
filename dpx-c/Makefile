NAME=libdpx
VERSION := $(shell git describe --abbrev=4 --dirty --always)

SHARED=$(NAME).so
STATIC=$(NAME).a

DESTDIR=
PREFIX=/usr/local

all: $(SHARED) $(STATIC)

OFILES=\
	channel.o\
	conn.o\
	frame.o\
	net.o\
	peer.o\
	dpx.o

CC=gcc
CFLAGS=-Wall -Werror=implicit-function-declaration -I. -ggdb -llthread -lltchan -lmsgpack -pthread

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c

$(SHARED): $(OFILES)
	$(CC) $(CFLAGS) -shared -o $(SHARED) $(OFILES)

$(STATIC): $(OFILES)
	ar rcs $(STATIC) $(OFILES)

clean:
	rm -f *.o $(SHARED) $(STATIC)
	$(MAKE) -C tests clean

check: all
	$(MAKE) -C tests check

install: $(SHARED) $(STATIC)
	cp $(SHARED) $(DESTDIR)$(PREFIX)/lib
	cp $(STATIC) $(DESTDIR)$(PREFIX)/lib
	cp *.h $(DESTDIR)$(PREFIX)/include

dist: clean
	git archive --format=tar --prefix=$(NAME)-$(VERSION)/ HEAD | xz -9v > $(NAME)-$(VERSION).tar.xz

.PHONY: all clean
