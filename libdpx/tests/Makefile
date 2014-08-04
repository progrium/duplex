BINARY=check_dpx

OFILES=\
	check.o

$(BINARY): $(OFILES)
	$(CC) -o $(BINARY) $(OFILES) $(CFLAGS)

check: $(BINARY)
	./check_dpx

CC=gcc
CFLAGS=-Wall -I. -I.. -ggdb -Wl,-rpath .. -L.. -ldpx -ltask -lmsgpack -pthread `pkg-config --cflags --libs check`

%.o: %.c
	$(CC) -c $*.c $(CFLAGS)

clean:
	rm -f *.o
	rm -f check_dpx

.PHONY: check
