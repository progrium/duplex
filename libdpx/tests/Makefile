BINARY=check_dpx

OFILES=\
	check.o

$(BINARY): $(OFILES)
	$(CC) $(CFLAGS) -o $(BINARY) $(OFILES)

check: $(BINARY)
	./check_dpx

CC=gcc
CFLAGS=-Wall -I. -I.. -ggdb -llthread -lltchan -lmsgpack -L../ -Wl,-rpath .. -ldpx -pthread `pkg-config --cflags --libs check`

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c

clean:
	rm -f *.o
	rm -f check_dpx

.PHONY: check