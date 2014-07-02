NAME=libdpx
VERSION := $(shell git describe --abbrev=4 --dirty --always)

SHARED=$(NAME).so
STATIC=$(NAME).a

DESTDIR=
PREFIX=/usr/local

all: task $(SHARED) $(STATIC)

OFILES=\
	channel.o\
	conn.o\
	frame.o\
	peer.o \
	dpx.o

CC=gcc
CFLAGS=-Wall -I. -Ilibtask -ggdb -ltask -lmsgpack -pthread

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c

$(SHARED): $(OFILES)
	$(CC) $(CFLAGS) -shared -o $(SHARED) $(OFILES)

$(STATIC): $(OFILES)
	ar rcs $(STATIC) $(OFILES)

task:
	$(MAKE) -C libtask

task_install:
	$(MAKE) -C libtask install

clean:
	rm -f *.o $(SHARED) $(STATIC)
	$(MAKE) -C libtask clean
	$(MAKE) -C tests clean

check: all
	$(MAKE) -C tests check

install: libtask $(SHARED) $(STATIC)
	cp $(SHARED) $(DESTDIR)$(PREFIX)/lib
	cp $(STATIC) $(DESTDIR)$(PREFIX)/lib
	cp *.h $(DESTDIR)$(PREFIX)/include

dist: clean
	git archive --format=tar --prefix=$(NAME)-$(VERSION)/ HEAD | xz -9v > $(NAME)-$(VERSION).tar.xz

.PHONY: all clean
