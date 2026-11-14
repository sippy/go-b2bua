# Embedded rtp.io engine (sippy/rtp_proxy/embedded)

In-process RTPProxy backend ("rtp.io" architecture): the actual upstream
RTPProxy engine (`librtpproxy.a`) is statically linked into the Go process
and started/stopped through the upstream C API
(`rtpp_main` / `rtpp_shutdown` from `src/librtpproxy.h`). The B2BUA / phone
keep using their normal `RtpProxyClient`: the in-process engine listens on
the same control sockets (unix/udp/tcp specs) as a standalone `rtpproxy`,
so external RTPProxy clients remain fully backward-compatible.

## Selecting the backend (Python-B2BUA-compatible)

The in-process engine is selected through the EXISTING RTP-proxy address
option — no separate runtime flags:

- `b2bua_radius`: `rtp_proxy_clients=rtp.io:...`
- `webrtc_phone`: `-rtpp rtp.io:...`

The `rtp.io:` spec is parsed with the same tokenization as the Python B2BUA
reference (`python-b2bua sippy/Rtp_proxy/client.py __init_internal`:
`proto, rest = addr.split(':', 1)`, `extra_args = rest.split(';')`; and
`sippy/Rtp_proxy/Client/internal.py __init`: the first `modules=` token
supplies the static module list, value split on `+`; every `modules=` token
is consumed; remaining tokens are raw engine arguments, each token being one
engine argv element):

| spec | meaning |
|---|---|
| `rtp.io:` | in-process engine, defaults |
| `rtp.io:modules=dtls_gw+ice_lite` | + static modules (DTLS gateway + ICE lite) |
| `rtp.io:modules=dtls_gw+ice_lite;-l;0.0.0.0;-m;12000;-M;15000` | + raw engine args (one per `;` token) |
| `unix:/path`, `udp:h:p`, `tcp:h:p` | external rtpproxy (unchanged behavior) |

Rules (matching the reference):

- only the `rtp.io:` prefix (with colon) selects the in-process engine;
  `rtp.io` without a colon is treated as a (failing) unix path, like the
  reference;
- the first `modules=` token wins; all `modules=` tokens are removed from
  the engine args;
- at most ONE `rtp.io:` entry per process (the engine is process-global, no
  restart); multiple entries are a clear startup error;
- a `rtp.io:` selection in a binary built WITHOUT `-tags rtpio` is a clear,
  actionable startup error (no silent fallback to external).

Embedding-process additions (on top of the user's tokens):

- the B2BUA/phone generates a private unix control socket
  (`/tmp/go-b2bua-rtpp-<pid>.sock` / `/tmp/go-webrtc-phone-rtpp-<pid>.sock`)
  and passes it to the engine as `-s`; the rtp.io: client entry is replaced
  by that socket, so the existing client code is untouched;
- `b2bua_radius` additionally passes the B2BUA's own command socket as the
  engine's no-media-timeout notification socket (`-n <b2bua_socket>` — the
  same target an external rtpproxy is given, so the B2BUA's timeout handling
  is unchanged);
- if the tokens contain no `-m`/`-M`, a default RTP port range
  (12000-15000) is injected; user-provided `-m`/`-M` win.

Divergence from the Python internal client (documented, technical): the
Python rtp.io C client has the B2BUA *connect* to the engine's own notify
socket (`nsetup_f`); here the engine *dials* the B2BUA's command socket via
`-n` — the observable event stream (B2BUA receives `r <id>` no-media
timeout notifications on its socket) is identical, and it matches the
external-rtpproxy topology the Go B2BUA already implements.

## Docker CI using the published RTPProxy archive

GitHub Actions uses the archive and header shipped in
`sippylabs/rtpproxy:latest` at `/usr/local/lib/librtpproxy.a` and
`/usr/local/include/librtpproxy.h`; it does not build RTPProxy from source.
The Go build runs in that same base image to match native dependencies.
The published archive can contain LLVM bitcode, so the whole-object conversion
uses Clang/LLD (`CC=clang LDFLAGS=-fuse-ld=lld`).

From the repository root:

```sh
docker build -t go-b2bua-embedded:ci -f sippy/rtp_proxy/embedded/build/Dockerfile.ci .
docker run --rm go-b2bua-embedded:ci
```

This builds both tagged commands and runs engine lifecycle, parser, and command
integration tests. It is not a browser media test. The source-build instructions
below remain available for pinned upstream development builds.

## Building

### 1. Pinned upstream archive (with DTLS/ICE static modules)

```
cd sippy/rtp_proxy/embedded/build
docker build -t rtpproxy-a:640f71a \
  --build-arg RTPPROXY_REF=640f71a1fd6e346f728f3e498e2e96517e2ba406 \
  -f rtpproxy/Dockerfile .
docker create --name rtpproxy-a rtpproxy-a:640f71a
docker cp rtpproxy-a:/opt/rtpproxy-lib/librtpproxy.a ../lib/
docker cp rtpproxy-a:/opt/rtpproxy-lib/librtpproxy.h ../include/
docker cp rtpproxy-a:/opt/rtpproxy-lib/configure.log .   # build evidence
docker rm -f rtpproxy-a
bash make_whole.sh ../lib        # -> librtpproxy_whole.a
```

The pinned build **must** install `libsrtp2-dev` + `libssl-dev` (the recipe
does): configure sets `BUILD_CRYPTO` only when libsrtp2 is detected, and
only then does `src/Makefile.am` add the `dtls_gw`/`ice_lite` module
objects to the archive. Without them the static modules are silently absent.

### 2. The whole-object archive (why `librtpproxy_whole.a`)

The static modules register themselves only through linker-set section data
(`DATA_SET` in `rtpp_dtls_gw.c` / `rtpp_ice_lite.c`); no symbol references
them, so a normal per-member archive link drops those objects and the
engine reports `static module is not compiled in` (verified: even the
upstream CLI `rtpproxy` binary lacks the section under plain archive
linking). Go's cgo LDFLAGS reject `-Wl,--whole-archive`, so `make_whole.sh`
pre-builds the equivalent: all archive objects → one relocatable object
(`gcc -r`) → single-member archive. Pulling in `rtpp_main` then pulls in
the whole engine **including** the module registrations.

### 3. Go

```
go build -tags rtpio ./cmd/b2bua_radius
go build -tags rtpio ./cmd/webrtc_phone
go test  -tags rtpio ./sippy/rtp_proxy/embedded/   # parse/equiv + lifecycle + guards
```

Untagged builds compile a stub (`ErrNotBuilt`) so all existing code paths
keep working; an `rtp.io:` selection in an untagged binary fails fast with
a clear message.

## Examples

```
# b2bua_radius: in-process engine with DTLS/ICE, extra engine args:
b2bua_radius -rtp_proxy_clients=rtp.io:modules=dtls_gw+ice_lite;-l;0.0.0.0;-m;12000;-M;15000 ...

# b2bua_radius: mixed in-process + external (allowed; engine serves only
# the rtp.io: entry):
b2bua_radius -rtp_proxy_clients=rtp.io:modules=dtls_gw+ice_lite,udp:127.0.0.1:5001 ...

# webrtc_phone (see examples/webrtc_phone/Dockerfile.rtpio + run_rtpio.sh):
go-webrtc-phone -rtpp "rtp.io:modules=dtls_gw+ice_lite;-l;172.18.0.3" ...
```

## API / lifecycle constraints (upstream)

- The engine is **process-global**: one engine per process; the wrapper
  enforces this with an explicit state machine: `Start` after `Stop` fails
  (no restart), `Stop` before/after `Stop` is a no-op, and invalid argv
  (nil / empty elements) is rejected before entering C.
- `rtpp_main()` performs the full engine startup synchronously and returns
  the configured engine (library mode: `_rtpp_main` returns instead of
  running the CLI event loop); the engine's internal elPeriodic threads run
  the media event loop.
- The C side owns the `rtpp_cfg*` **and the argv strings**: upstream
  `main.c` retains `optarg` pointers (`cfsp->cfile/rdir/sdir/pid_file/
  runcreds->uname`) for the engine lifetime, so the wrapper keeps the whole
  argv vector + all strings C-allocated until `Stop()` frees them. No
  Go-owned memory is ever passed to the engine, and there are no C→Go
  callbacks.
- Module load failure is FATAL in the upstream engine (`IC_ERRX` →
  `exit(1)`): a failed `--dso` kills the whole Go process.
- The in-process engine needs the same runtime arguments a standalone
  rtpproxy gets (listen address, `-W`/`-T` session timeouts, and the `-n`
  no-media timeout notification the B2BUA listens on) — without them
  no-media-timeout sessions are not torn down.

## System dependencies

`librtpproxy_whole.a` links with `-lm -lpthread -ldl -lsrtp2 -lssl
-lcrypto` (the latter three only matter once the DTLS static module is in
the archive). Engine-internal deps (libucl, libxxHash, elPeriodic, ...) are
carried inside the archive.
