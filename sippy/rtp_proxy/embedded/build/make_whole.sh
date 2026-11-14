#!/bin/bash
# Produce librtpproxy_whole.a from upstream librtpproxy.a.
#
# Why: the DTLS-gateway / ICE-lite static modules register themselves only
# through linker-set section data (DATA_SET in rtpp_dtls_gw.c /
# rtpp_ice_lite.c); no symbol references them, so a normal per-member
# archive link drops those objects and the engine reports 'static module
# is not compiled in' (verified: the upstream CLI rtpproxy binary also
# lacks the section under plain archive linking). Go's cgo LDFLAGS reject
# -Wl,--whole-archive, so the equivalent is pre-built here: all archive
# objects are combined into one relocatable object (gcc -r) and wrapped in
# a single-member archive. When the Go binary pulls in rtpp_main, it pulls
# in the ENTIRE engine + all module registrations.
#
# Usage: make_whole.sh <dir-with-librtpproxy.a>   (writes into that dir)
set -eu
DIR="${1:?usage: make_whole.sh <dir-with-librtpproxy.a>}"
DIR="$(cd "${DIR}" && pwd)"   # resolve relative paths BEFORE the mktemp cd below
A="${DIR}/librtpproxy.a"
test -f "${A}" || { echo "missing ${A} (build it with build/rtpproxy/Dockerfile first)"; exit 1; }
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT
( cd "${TMP}" && ar x "${A}" && ${CC:-gcc} ${LDFLAGS:-} -r *.o -o "${TMP}/librtpproxy_whole.o" \
 && ar rcs "${DIR}/librtpproxy_whole.a" "${TMP}/librtpproxy_whole.o" )
echo "wrote ${DIR}/librtpproxy_whole.a"
nm "${DIR}/librtpproxy_whole.a" | grep -cE "rtpp_dtls|rtpp_ice" | sed 's/^/  dtls+ice symbols: /'
