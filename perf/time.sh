#!/usr/bin/bash
cd perf
pushd .
echo "Creating 250GB file for throughput timing..."
dd if=/dev/zero of=/tmp/file  bs=1024 count=250000

# set up http server
cd /tmp
  python -m SimpleHTTPServer > /dev/null 2>&1 &
  echo $! &> /tmp/http.pid
popd

# haproxy if available
haproxy -f haproxy.conf -dk -dp -db 2>&1 &
echo $! &> /tmp/haproxy.pid

# pure python
python proxy.py 8000 2>&1 &
echo $! &> /tmp/python.pid

# perform timing
sleep 0.5
echo "\033[31m"
(time curl http://localhost:8000/file) 2>&1 | grep real | sed 's/real/direct/'
(time curl http://localhost:9000/file) 2>&1 | grep real | sed 's/real/haproxy/'
(time curl http://localhost:10000/file) 2>&1 | grep real | sed 's/real/python/'
echo "\033[0m"

# cleanup
cat /tmp/haproxy.pid | xargs kill -9 
cat /tmp/python.pid | xargs kill -9 
cat /tmp/http.pid | xargs kill -9 
rm /tmp/file
