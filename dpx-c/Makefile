NAME=dpx-c
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
	peer.o

CC=gcc
CFLAGS=-Wall -I. -ggdb -ltask -lmsgpack

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c

$(SHARED): $(OFILES)
	$(CC) $(CFLAGS) -shared -o $(SHARED) $(OFILES)

$(STATIC): $(OFILES)
	ar rcs $(STATIC) $(OFILES)

clean:
	rm -f *.o $(SHARED) $(STATIC)

install: $(SHARED) $(STATIC)
	cp $(SHARED) $(DESTDIR)$(PREFIX)/lib
	cp $(STATIC) $(DESTDIR)$(PREFIX)/lib
	cp *.h $(DESTDIR)$(PREFIX)/include

dist: clean
	git archive --format=tar --prefix=$(NAME)-$(VERSION)/ HEAD | xz -9v > $(NAME)-$(VERSION).tar.xz

.PHONY: all clean
