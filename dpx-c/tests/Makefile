BINARY=check_dpx

OFILES=\
	dpx.o

$(BINARY): $(OFILES)
	$(CC) $(CFLAGS) -o $(BINARY) $(OFILES)

check: $(BINARY)
	./check_dpx

CC=gcc
CFLAGS=-Wall -I. -I.. -I../libtask -ggdb -ltask -lmsgpack -L.. -L../libtask -Wl,-rpath ../libtask -Wl,-rpath .. -ldpx -fcheck=all -pthread `pkg-config --cflags --libs check`

%.o: %.c
	$(CC) $(CFLAGS) -fPIC -c $*.c

clean:
	rm -f *.o
	rm -f check_dpx

.PHONY: check