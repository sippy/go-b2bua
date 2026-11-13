# syntax=docker/dockerfile:1
# Reproducible pinned build of the upstream librtpproxy.a (in-process
# rtp.io engine library) with DTLS-gateway + ICE-lite STATIC modules.
#
# Why libsrtp2-dev + libssl-dev are required (ST2 owner-verified):
#   configure.ac sets BUILD_CRYPTO when libsrtp2 is detected; with
#   BUILD_CRYPTO, src/Makefile.am adds modules/dtls_gw and modules/ice_lite
#   to librtpproxy_la_LIBADD, so the static modules are compiled INTO the
#   archive. Without libsrtp2-dev the modules are silently absent.
#
# Usage:
#   docker build -t rtpproxy-a:<ver> -f rtpproxy/Dockerfile \
#     --build-arg RTPPROXY_REF=<full-sha> .   # from this build/ dir
#   docker create --name rtpproxy-a rtpproxy-a:<ver>
#   docker cp rtpproxy-a:/opt/rtpproxy-lib/librtpproxy.a   <repo>/sippy/rtp_proxy/embedded/lib/
#   docker cp rtpproxy-a:/opt/rtpproxy-lib/librtpproxy.h   <repo>/sippy/rtp_proxy/embedded/include/
#   docker cp rtpproxy-a:/opt/rtpproxy-lib/configure.log   # evidence (kept for review)
#   docker rm -f rtpproxy-a
#   bash make_whole.sh <repo>/sippy/rtp_proxy/embedded/lib   # produces librtpproxy_whole.a
ARG RTPPROXY_REF=640f71a1fd6e346f728f3e498e2e96517e2ba406
FROM debian:bookworm
ARG RTPPROXY_REF=640f71a1fd6e346f728f3e498e2e96517e2ba406
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential autoconf automake libtool pkg-config git ca-certificates \
      libssl-dev libsrtp2-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /build
# Fresh pinned clone: no local/submodule/experimental state can leak in.
RUN git init rtpproxy && cd rtpproxy \
 && git remote add origin https://github.com/sippy/rtpproxy.git \
 && git fetch --depth 1 origin ${RTPPROXY_REF} \
 && git checkout FETCH_HEAD \
 && git rev-parse HEAD | grep -qx "${RTPPROXY_REF}" || { echo "PIN MISMATCH"; exit 1; }
RUN cd /build/rtpproxy \
 && ./configure --enable-librtpproxy > /tmp/configure.log 2>&1 \
 && echo "=== configure feature evidence (srtp/ssl/crypto) ===" \
 && grep -nE "checking for srtp|checking.*ssl|checking.*crypto" /tmp/configure.log \
 && make -j8 > /tmp/make.log 2>&1 \
 && A=$(ls src/.libs/librtpproxy.a src/librtpproxy.a 2>/dev/null | head -1) \
 && test -n "${A}" \
 && mkdir -p /opt/rtpproxy-lib \
 && cp "${A}" /opt/rtpproxy-lib/librtpproxy.a \
 && cp src/librtpproxy.h /opt/rtpproxy-lib/ \
 && cp /tmp/configure.log /opt/rtpproxy-lib/configure.log \
 && echo "=== archive ===" && ls -la /opt/rtpproxy-lib \
 && echo "=== engine API symbols ===" \
 && nm /opt/rtpproxy-lib/librtpproxy.a | grep -E " T (rtpp_main|rtpp_shutdown)$" \
 && echo "=== static DTLS/ICE module objects (must be non-zero) ===" \
 && nm /opt/rtpproxy-lib/librtpproxy.a | grep -cE "rtpp_dtls|rtpp_ice"
