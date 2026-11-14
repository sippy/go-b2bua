[![Go Build](https://github.com/sippy/go-b2bua/actions/workflows/go.yml/badge.svg)](https://github.com/sippy/go-b2bua/actions/workflows/go.yml)

# Go B2BUA

Go B2BUA is a Go port of the [Sippy B2BUA](https://github.com/sippy/b2bua).

## Differences from the Python B2BUA

- Smaller memory footprint (approximately 2.5x).
- All available CPU cores are utilized.
- Runs faster (approximately 4x per CPU core).
- The configuration object is not global, allowing several B2BUA instances
  inside one executable. The optional embedded RTP engine has a separate
  one-engine-per-process restriction.

## WebRTC and RTP media support

- SIP over secure WebSocket (WSS), with connection-aware transaction routing.
- RTPProxy DTLS gateway and ICE-lite integration, including SDP conversion
  between browser WebRTC media and plain SIP-side RTP.
- A SIP.js browser phone and Go gateway demo, with Docker recipes for either
  an external RTPProxy or a statically linked, in-process **rtp.io** engine.
- Optional embedded RTPProxy support in `b2bua_radius` and `webrtc_phone`.
  Existing external RTPProxy configurations remain supported.

The embedded backend requires a cgo build with `-tags rtpio`, the pinned
RTPProxy static archive, and its native dependencies. Select it through the
existing Python-compatible configuration: `rtp_proxy_clients=rtp.io:...` in
`b2bua_radius`, or `-rtpp 'rtp.io:...'` in the phone. Semicolons separate
engine arguments; `modules=dtls_gw+ice_lite` selects the static media modules.
There are no separate `-rtpio*` runtime flags. Untagged builds retain external
RTPProxy support and reject an `rtp.io:` selection with an explicit error.
The embedded engine is process-global and cannot restart after shutdown.

## Documentation

- [WebRTC phone guide](examples/webrtc_phone/README.md): Docker build and
  launch instructions, TLS/WSS setup, SIP routing, LAN configuration, and
  automated browser call checks. Build the pinned archive described below
  **before** building the in-process phone image.
- [Embedded rtp.io engine](sippy/rtp_proxy/embedded/README.md): pinned archive
  build, static-module linking, configuration syntax, native dependencies,
  and lifecycle constraints.

## Build and verification

The repository uses a Go workspace (`go.work`). The standard `make` and
`make test` targets build the existing command examples and run the selected
SIP/RTP-proxy test suites; they do not build the tagged embedded engine or run
WebRTC browser tests. GitHub Actions runs those standard targets on Go 1.21.1
through 1.25.x. A separate Docker CI job uses the static RTPProxy archive
from the published Docker Hub image, builds both embedded commands on Go 1.25, and runs
the tagged engine and command tests (including the real engine lifecycle).
VoIPTests CI also runs the upstream debug and production SIP call batteries
against this checkout with external RTPProxy over Unix, UDP, and TCP control
sockets, plus the in-process `rtp.io:` backend linked from the Docker Hub
archive (no separate RTPProxy process), uploading logs for each backend.
Browser call tests remain separate; use the Docker functional-test instructions
in the linked WebRTC guide.

The in-process Docker demo has been verified with two outbound calls and one
inbound call, DTLS connected and bidirectional RTP packet flow on every call.
This does not establish subjective audio quality, TURN/NAT traversal,
transcoding, or multi-stream interoperability. The demo is an unauthenticated
gateway with self-signed certificates, not a production-hardened deployment.
