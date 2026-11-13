#!/bin/sh
set -eu
rtpp_pid= b2b_pid=
cleanup() {
  trap - EXIT INT TERM
  [ -z "$b2b_pid" ] || kill "$b2b_pid" 2>/dev/null || true
  [ -z "$rtpp_pid" ] || kill "$rtpp_pid" 2>/dev/null || true
  wait || true
}
trap cleanup EXIT INT TERM
/usr/local/bin/rtpproxy -f -F -s /tmp/go-webrtc-rtpp.sock \
  -l "$RTP_ADDRESS" -m "$MIN_RTP_PORT" -M "$MAX_RTP_PORT" -d "${RTPP_LOG_LEVEL:-INFO}" \
  --dso /usr/local/lib/rtpproxy/rtpp_dtls_gw.so \
  --dso /usr/local/lib/rtpproxy/rtpp_ice_lite.so &
rtpp_pid=$!
# Socket creation is a readiness check, not a test of the application.
count=0
while [ ! -S /tmp/go-webrtc-rtpp.sock ]; do
  count=$((count + 1)); [ "$count" -lt 50 ] || exit 1
  sleep 0.1
done
/usr/local/bin/go-webrtc-phone \
  -sip "$SIP_ADDRESS" -next-hop "${OUTBOUND_ROUTE:-$OUTBOUND_PROXY}" -registrar "$OUTBOUND_PROXY" \
  -https "$HTTPS_ADDRESS" -wss "$WSS_ADDRESS" \
  -cert tls/server.crt -key tls/server.key -web web -rtpp /tmp/go-webrtc-rtpp.sock &
b2b_pid=$!
wait "$b2b_pid"
