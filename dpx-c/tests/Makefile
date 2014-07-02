check: $(OFILES)

OFILES=\
	dpx.o

CC=gcc
CFLAGS=-Wall -I. -I.. -I../libtask -ggdb -ltask -lmsgpack -pthread

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c
