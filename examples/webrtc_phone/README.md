# Go WebRTC phone

This is the Go counterpart of [sippy/webrtc_phone](https://github.com/sippy/webrtc_phone):
SIP.js in the browser, SIP/WSS in Go, and RTPProxy's DTLS gateway and ICE-lite modules.
The image uses `sippylabs/rtpproxy:latest` as its runtime base. SIP.js is packaged at
build time, not loaded from a public CDN at runtime. There is no Python B2BUA.

## Build and launch locally

From the go-b2bua repository root:

```sh
docker pull sippylabs/rtpproxy:latest
docker build -f examples/webrtc_phone/Dockerfile.rtpio -t go-webrtc-phone:local .
docker run -d --name go-webrtc-phone --network host go-webrtc-phone:local
```

The default demo uses the statically linked, in-process rtp.io engine. Build the
pinned archive first using [the embedded build instructions](../../sippy/rtp_proxy/embedded/README.md).
The launcher selects it through the existing Python-compatible RTP-proxy option:

```sh
-rtpp 'rtp.io:modules=dtls_gw+ice_lite;-l;127.0.0.1;-m;35000;-M;35100'
```

Each semicolon-delimited token is one engine argument; module names are separated
by `+`. The launcher substitutes `RTP_ADDRESS`, `MIN_RTP_PORT` and `MAX_RTP_PORT`.
There is no separate RTPproxy process or `-rtpio*` runtime flag. The original
`Dockerfile` remains available for an external-RTPproxy demo.

Open <https://localhost:18443/>. Accept the local self-signed demo certificate,
then visit <https://localhost:19876/> once to trust the WSS listener's certificate.
Return to the phone, **Connect**, optionally **Register**, then **Call**.
No outbound SIP server is bundled in the phone image: configure your SIP service
or start the disposable echo peer below. HTTPS/WSS defaults are 18443/19876;
SIP/UDP is 15060, RTP UDP range is 35000–35100.

For another SIP service, supply `OUTBOUND_PROXY=host:port`. `OUTBOUND_ROUTE` may
override the call next hop separately; REGISTER always uses `OUTBOUND_PROXY`.
Unlike the Python radius B2BUA's rich static-route syntax, this small demo
accepts a single `host:port` next hop, including `[IPv6]:port` syntax.

Default media address is loopback for a same-host demonstration. To use a browser
on another machine, set `RTP_ADDRESS` and `SIP_ADDRESS` to the Docker host's reachable
address (for example `RTP_ADDRESS=192.168.1.10 SIP_ADDRESS=192.168.1.10:15060`),
adjust the WSS field in the page if ports changed, and supply trusted TLS
certificate/key files by mounting `/opt/go-webrtc-phone/tls`. Host networking is
intentional: avoid translating the entire RTP range or rewriting ICE candidates.
The listener is an unauthenticated demo gateway; SIP registration credentials go
to the configured registrar. It is not a deployment-hardening configuration.

## Docker-only functional proof

The disposable peer implements REGISTER, SIP/UDP calls and RTP packet echo. It
is not a replacement PBX. The automated browser uses fake audio so no microphone
or interactive permission prompt is needed.

```sh
docker build -f examples/webrtc_phone/tests/Dockerfile -t go-webrtc-tests:local .
docker run -d --name go-webrtc-peer --network host go-webrtc-tests:local node peer.cjs
mkdir -p /tmp/go-webrtc-evidence
docker run --rm --network host \
  -v /tmp/go-webrtc-evidence:/evidence go-webrtc-tests:local
```

Acceptance checks: WSS connection, REGISTER proxy, two real browser calls,
ICE and DTLS connected, outbound and inbound RTP counters increasing, plain
RTP/AVP SDP without browser ICE/DTLS attributes on the SIP side, browser-initiated
BYE, and remote SIP-initiated BYE. Raw browser stats, peer counts and a screenshot
are written to the evidence volume. This proves packet flow, not subjective
audio quality, transcoding, TURN traversal or every SIP.js demo feature.

```sh
docker logs go-webrtc-phone
docker stop go-webrtc-phone go-webrtc-peer
docker rm go-webrtc-phone go-webrtc-peer
```

## Change batches

The transport and media library changes are independent batches; this demo is a
third integration batch. Any general Python/Go parity corrections discovered
during the port are documented separately from WebRTC-specific support.

## Validated LAN launch (2026-11-13)

The phone uses the in-process rtp.io launcher and the disposable echo peer on this host:
<https://192.168.23.109:18443/> (WSS trust endpoint
<https://192.168.23.109:19876/>). Accept the local demo certificates, Connect,
Register, Call `sip:echo@localhost`. Microphone permission is required for manual
calls. Reachability from another computer has not been independently checked.

Equivalent phone command after building and starting the echo peer:

```sh
docker run -d --name go-webrtc-phone --network host \
  -e RTP_ADDRESS=192.168.23.109 -e SIP_ADDRESS=192.168.23.109:15060 \
  go-webrtc-phone:local
```

For the three-call suite against that configuration:

```sh
docker run --rm --network host -e INBOUND_TEST=1 -e RTP_ADDRESS=192.168.23.109 \
  -v /tmp/go-webrtc-evidence:/evidence go-webrtc-tests:local
```

This suite checks bidirectional packet counters on **each** call, not merely
aggregate peer activity.

The updated in-process LAN demo passed this suite on 2026-11-13: two outbound
calls and one inbound call, DTLS connected on all three, with bidirectional RTP
packet counts of 30/30, 31/31 and 21/21. The existing TLS certificates and echo
peer were retained during the update.
