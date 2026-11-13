//
// Copyright (c) 2026 Sippy Software, Inc. All rights reserved.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
// list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation and/or
// other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
// ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//

//go:build !rtpio

// Embedded rtp.io engine backend (rtp.io integration, rtpio-20261113 ST1).
//
// This package wraps the upstream RTPProxy C engine (librtpproxy.a) so that a
// B2BUA can run the RTPProxy in-process (the "rtp.io" architecture) instead of
// talking to a standalone rtpproxy child process. The existing
// RtpProxyClient(s) keep working unchanged: the in-process engine listens on
// the same sockets (unix/udp/tcp specs) the clients already use.
//
// Build modes:
//   - untagged build (default): this stub is compiled; Start() returns
//     ErrNotBuilt so commands fail fast with a clear message, and all other
//     code paths (external RTPProxy) stay fully functional.
//   - `go build -tags rtpio`: the cgo implementation (embedded_cgo.go)
//     statically links librtpproxy.a from this package's lib/ directory
//     (drop the upstream-built archive and header into lib/ and include/
//     before building).
//
// Lifecycle constraints (upstream API):
//   - The engine is process-global: at most one Start per process.
//   - No restart: once stopped the engine cannot be re-created (upstream
//     keeps process-wide engine state).
//   - Start() performs the full engine startup synchronously (in library
//     mode the upstream _rtpp_main returns the configured engine instead of
//     running the CLI event loop); Stop() calls rtpp_shutdown.
package embedded

import "errors"

// ErrNotBuilt is returned by Start() when the binary was built without the
// rtpio build tag (or without librtpproxy.a present at link time).
var ErrNotBuilt = errors.New(
    "embedded rtp.io engine not available: build with -tags rtpio and " +
    "librtpproxy.a in sippy/rtp_proxy/embedded/lib/ (see package README)")

// Available reports whether the embedded rtp.io engine was compiled in.
func Available() bool { return false }

// Start is a stub in untagged builds; it never starts an engine and always
// returns ErrNotBuilt (no state is tracked).
func Start(argv []string) error {
    if len(argv) == 0 {
        return errors.New("embedded rtp.io engine: empty argv")
    }
    return ErrNotBuilt
}

// Stop is a no-op in untagged builds.
func Stop() error { return nil }
