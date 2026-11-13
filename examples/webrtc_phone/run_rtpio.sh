#!/bin/sh
set -eu
# go-webrtc-phone launcher for the IN-PROCESS rtp.io engine.
# No standalone rtpproxy process: the statically linked engine starts inside
# the Go process on a generated control socket (the client connects to it
# like an external rtpproxy). The DTLS-gateway and ICE-lite STATIC modules
# (compiled into librtpproxy.a when srtp2+OpenSSL were detected at archive
# build time) are selected by NAME, Python-B2BUA-compatible:
#   -rtpp "rtp.io:modules=dtls_gw+ice_lite;-l;$RTP_ADDRESS;-m;$MIN_RTP_PORT;-M;$MAX_RTP_PORT"
# (each ";" token is one engine argv element; see sippy/rtp_proxy/embedded/README.md)
MIN_RTP_PORT="${MIN_RTP_PORT:-12000}"
MAX_RTP_PORT="${MAX_RTP_PORT:-15000}"
exec /usr/local/bin/go-webrtc-phone \
  -sip "$SIP_ADDRESS" -next-hop "${OUTBOUND_ROUTE:-$OUTBOUND_PROXY}" -registrar "$OUTBOUND_PROXY" \
  -https "$HTTPS_ADDRESS" -wss "$WSS_ADDRESS" \
  -cert tls/server.crt -key tls/server.key -web web \
  -rtpp "rtp.io:modules=dtls_gw+ice_lite;-l;$RTP_ADDRESS;-m;$MIN_RTP_PORT;-M;$MAX_RTP_PORT"
