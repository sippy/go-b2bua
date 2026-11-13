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
//go:build rtpio

package embedded

import (
    "net"
    "os"
    "strings"
    "testing"
    "time"
)

// TestLifecycle validates the in-process engine start/stop contract against
// the actual upstream library, selected through the canonical Python-compatible
// rtp.io: spec (with the DTLS-gateway and ICE-lite STATIC modules loaded —
// the upstream engine exits the process on module load failure, so surviving
// Start() proves the static table resolved both modules): the control socket
// must appear while running, answer a version command, disappear after Stop,
// and a second Start must fail (process-global, no restart).
func TestLifecycle(t *testing.T) {
    if !Available() {
        t.Fatal("embedded engine not compiled in")
    }
    sock := "/tmp/rtpio_test_" + string(rune('a'+os.Getpid()&31)) + ".sock"
    internal, spec, err := ParseRtpIO("rtp.io:modules=dtls_gw+ice_lite;-l;127.0.0.1")
    if !internal || err != nil {
        t.Fatalf("ParseRtpIO canonical spec: internal=%v err=%v", internal, err)
    }
    argv := EngineArgv(spec, []string{"unix:" + sock}, "", 12000, 12500)
    if err := Start(argv); err != nil {
        t.Fatalf("Start failed: %v", err)
    }
    // Control socket must exist while the engine runs.
    if _, err := os.Stat(sock); err != nil {
        t.Fatalf("control socket %s missing after Start: %v", sock, err)
    }
    // Engine must answer a version command on the control socket.
    conn, err := net.Dial("unix", sock)
    if err != nil {
        t.Fatalf("dial control socket: %v", err)
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(5 * time.Second))
    if _, err = conn.Write([]byte("V\n")); err != nil {
        t.Fatalf("send V: %v", err)
    }
    buf := make([]byte, 64)
    n, err := conn.Read(buf)
    if err != nil {
        t.Fatalf("read version reply: %v", err)
    }
    if !strings.Contains(string(buf[:n]), "20") {
        t.Fatalf("unexpected version reply %q", string(buf[:n]))
    }
    // Stop must clean up: engine stopped, socket removed.
    if err := Stop(); err != nil {
        t.Fatalf("Stop failed: %v", err)
    }
    if err := Stop(); err != nil {
        t.Fatalf("second Stop must be a no-op: %v", err)
    }
    if _, err := os.Stat(sock); !os.IsNotExist(err) {
        t.Fatalf("control socket %s still present after Stop (err=%v)", sock, err)
    }
    // No restart: Start after Stop must fail.
    if err := Start(argv); err == nil {
        t.Fatal("Start after Stop must fail (no restart)")
    }
}

// TestArgvGuards validates invalid argv is rejected before entering C and
// without consuming the one-shot engine lifecycle (the real start/stop is
// covered by TestLifecycle, which runs in the same process).
func TestArgvGuards(t *testing.T) {
    if err := Start(nil); err == nil {
        t.Fatal("Start(nil) must fail")
    }
    if err := Start([]string{"rtpproxy", ""}); err == nil {
        t.Fatal("Start with empty element must fail")
    }
    if err := Start([]string{""}); err == nil {
        t.Fatal("Start with empty program name must fail")
    }
}
