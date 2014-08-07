NAME=libdpx
VERSION := $(shell git describe --abbrev=4 --dirty --always)

SHARED=$(NAME).so
STATIC=$(NAME).a

DESTDIR ?=
PREFIX ?= /usr/local

all: vendor $(STATIC) $(SHARED)

OFILES=\
	alchan.o\
	channel.o\
	conn.o\
	frame.o\
	peer.o\
	dpx.o

VENDORLIBS = $(sort $(wildcard vendor/.libs/*))

CC=gcc
CFLAGS ?=
OUR_CFLAGS = $(CFLAGS) -Wall -Werror=implicit-function-declaration -I. -ggdb

LDFLAGS ?=
OUR_LDFLAGS = $(LDFLAGS) -lmsgpack -pthread -Wl,--whole-archive $(VENDORLIBS) -Wl,--no-whole-archive

DEBUG ?= 0
ifeq ($(DEBUG), 1)
    OUR_CFLAGS += -DDEBUG
endif

ARCH := $(shell uname -m)
ifeq ($(ARCH), x86_64)
	OUR_CFLAGS += -fPIC
endif

vendor:
	$(MAKE) -C vendor all

%.o: %.c
	$(CC) -c $*.c $(OUR_CFLAGS)

$(SHARED): vendor $(OFILES)
	$(CC) -shared -o $(SHARED) $(OFILES) $(OUR_LDFLAGS) 

$(STATIC): vendor $(OFILES)
	ar rcs $(STATIC) $(OFILES)

clean:
	rm -f *.o $(SHARED) $(STATIC)
	$(MAKE) -C tests clean
	$(MAKE) -C vendor clean

check: all
	$(MAKE) -C tests check

install: all
	cp $(STATIC) $(DESTDIR)$(PREFIX)/lib
	cp $(SHARED) $(DESTDIR)$(PREFIX)/lib
	cp dpx.h $(DESTDIR)$(PREFIX)/include

dist: clean
	git archive --format=tar --prefix=$(NAME)-$(VERSION)/ HEAD | xz -9v > $(NAME)-$(VERSION).tar.xz

.PHONY: all clean vendor
