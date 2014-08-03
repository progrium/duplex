NAME=libdpx
VERSION := $(shell git describe --abbrev=4 --dirty --always)

SHARED=$(NAME).so
STATIC=$(NAME).a

DESTDIR=
PREFIX=/usr/local

all: $(STATIC) $(SHARED)

OFILES=\
	alchan.o\
	channel.o\
	conn.o\
	frame.o\
	peer.o\
	dpx.o

CC=gcc
CFLAGS=-Wall -Werror=implicit-function-declaration -I. -ggdb -ltask -lmsgpack -pthread

DEBUG ?= 0
ifeq ($(DEBUG), 1)
    CFLAGS += -DDEBUG
endif


%.o: %.c
	$(CC) -c $*.c $(CFLAGS) -fPIC

$(SHARED): $(OFILES)
	$(CC) -shared -o $(SHARED) $(OFILES) $(CFLAGS) 

$(STATIC): $(OFILES)
	ar rcs $(STATIC) $(OFILES)

clean:
	rm -f *.o $(SHARED) $(STATIC)
	$(MAKE) -C tests clean

check: all
	$(MAKE) -C tests check

install: $(STATIC) $(SHARED)
	cp $(STATIC) $(DESTDIR)$(PREFIX)/lib
	cp $(SHARED) $(DESTDIR)$(PREFIX)/lib
	cp dpx.h $(DESTDIR)$(PREFIX)/include

dist: clean
	git archive --format=tar --prefix=$(NAME)-$(VERSION)/ HEAD | xz -9v > $(NAME)-$(VERSION).tar.xz

.PHONY: all clean
