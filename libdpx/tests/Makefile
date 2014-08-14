CHECK_OFILES=\
	check.o

PERF_RECV_OFILES=\
	recv_lat.o

PERF_SEND_OFILES=\
	send_lat.o

all: check_dpx perf

check_dpx: $(CHECK_OFILES)
	$(CC) -o check_dpx $(CHECK_OFILES) $(CFLAGS)

perf: $(PERF_RECV_OFILES) $(PERF_SEND_OFILES)
	$(CC) -o recv_lat $(PERF_RECV_OFILES) $(CFLAGS)
	$(CC) -o send_lat $(PERF_SEND_OFILES) $(CFLAGS)

check: check_dpx
	./check_dpx

CC=gcc
CFLAGS=-Wall -I. -I.. -ggdb -Wl,-rpath .. -L.. -ldpx -pthread `pkg-config --cflags --libs check`

%.o: %.c
	$(CC) -c $*.c $(CFLAGS)

clean:
	rm -f *.o
	rm -f check_dpx
	rm -f recv_lat
	rm -f send_lat

.PHONY: check all
