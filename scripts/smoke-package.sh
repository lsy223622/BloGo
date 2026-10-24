#!/bin/sh
set -eu
archive=$1
for tool in go node python python3; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "Runtime smoke environment must not contain $tool" >&2
        exit 1
    fi
done
mkdir /tmp/blogo-smoke
tar -xzf "$archive" -C /tmp/blogo-smoke
cd /tmp/blogo-smoke/BloGo-*
./blog build --output independent
./blog serve > /tmp/blogo-smoke.log 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT
ready=false
for attempt in $(seq 1 100); do
    if wget -q -O /tmp/home http://127.0.0.1:25646/; then ready=true; break; fi
    sleep 0.1
done
if [ "$ready" != true ]; then cat /tmp/blogo-smoke.log; exit 1; fi
grep -q BloGo /tmp/home
wget -q -O /tmp/report http://127.0.0.1:25646/data/stats.json
if wget -q -O /tmp/private http://127.0.0.1:25646/data/private/report.html; then exit 1; fi
cp data/statistics/visitor.key /tmp/original-key
kill "$pid"
wait "$pid"
./blog serve > /tmp/blogo-smoke-restart.log 2>&1 &
pid=$!
ready=false
for attempt in $(seq 1 100); do
    if wget -q -O /tmp/home http://127.0.0.1:25646/; then ready=true; break; fi
    sleep 0.1
done
if [ "$ready" != true ]; then cat /tmp/blogo-smoke-restart.log; exit 1; fi
cmp data/statistics/visitor.key /tmp/original-key
kill "$pid"
wait "$pid"
trap - EXIT
echo "Package startup, build, privacy and restart passed as UID $(id -u)"
