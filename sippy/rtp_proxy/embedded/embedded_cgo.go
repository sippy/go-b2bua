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

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo LDFLAGS: -L${SRCDIR}/lib -lrtpproxy_whole -lm -lpthread -ldl -lsrtp2 -lssl -lcrypto
//
// -lrtpproxy_whole is a single-object static archive containing ALL objects
// of upstream librtpproxy.a (the build recipe produces it by
// `ar x` + `gcc -r`; see README). whole-archive equivalent is required:
// the DTLS/ICE static modules (compiled into librtpproxy.a when srtp2+OpenSSL
// are detected) register themselves only through linker-set section data
// (DATA_SET in rtpp_dtls_gw.c / rtpp_ice_lite.c); no symbol references them,
// so a normal per-member archive link would drop those objects and the
// engine would report 'static module is not compiled in'. (Verified: the
// upstream CLI rtpproxy binary also lacks the section under plain archive
// linking.) Go's cgo LDFLAGS rejects -Wl,--whole-archive, hence the recipe.
#include <stdlib.h>
#include "librtpproxy.h"

// The upstream engine is process-global; the C side owns the cfg pointer and
// (importantly) RETAINS the argv strings: _rtpp_main() keeps optarg pointers
// in cfsp->cfile/rdir/sdir/pid_file/runcreds->uname for the engine lifetime.
// The Go side therefore keeps the whole argv vector and all strings
// C-allocated and C-owned until Stop() frees them. No Go-owned memory is
// ever passed to the engine, and there are no C->Go callbacks.
static struct rtpp_cfg *g_rtpp_cfg = NULL;

static int rtpp_start(int argc, const char * const *argv)
{
    if (g_rtpp_cfg != NULL || argc <= 0 || argv == NULL || argv[0] == NULL) {
        return -1;
    }
    g_rtpp_cfg = rtpp_main(argc, argv);
    return (g_rtpp_cfg == NULL) ? -1 : 0;
}

static int rtpp_stop(void)
{
    if (g_rtpp_cfg == NULL) {
        return -1;
    }
    rtpp_shutdown(g_rtpp_cfg);
    g_rtpp_cfg = NULL;
    return 0;
}
*/
import "C"

import (
    "errors"
    "sync"
    "unsafe"
)

// Engine state (process-global): 0 = never started, 1 = running, 2 = stopped.
const (
    stateNever = 0
    stateRun   = 1
    stateStopped = 2
)

var (
    mu     sync.Mutex
    state  = stateNever
    cargvs []*C.char      // C-allocated arg strings, C-owned
    cvec   unsafe.Pointer // C-allocated argv vector, C-owned
)

// Available reports that the embedded rtp.io engine is compiled in.
func Available() bool { return true }

// Start launches the in-process RTPProxy engine with an rtpproxy-style
// command line (argv[0] is the program name, conventionally "rtpproxy").
// The argv strings are retained C-allocated until Stop(), because the
// upstream engine retains optarg pointers for its lifetime.
//
// Lifecycle: the engine is process-global; exactly one Start per process.
// Start after Stop fails (no restart); Stop before Start is a no-op.
func Start(argv []string) error {
    if len(argv) == 0 {
        return errors.New("embedded rtp.io engine: empty argv")
    }
    for _, a := range argv {
        if a == "" {
            return errors.New("embedded rtp.io engine: empty argv element")
        }
    }
    mu.Lock()
    defer mu.Unlock()
    if state == stateRun {
        return errors.New("embedded rtp.io engine already started")
    }
    if state == stateStopped {
        return errors.New("embedded rtp.io engine already stopped (process-global, no restart)")
    }
    cargvs = make([]*C.char, len(argv))
    for i, a := range argv {
        cargvs[i] = C.CString(a)
    }
    cvec = C.malloc(C.size_t(len(argv) + 1) * C.size_t(unsafe.Sizeof((*C.char)(nil))))
    vec := unsafe.Slice((**C.char)(cvec), len(argv)+1)
    for i := 0; i < len(argv); i++ {
        vec[i] = cargvs[i]
    }
    vec[len(argv)] = nil
    if rc := C.rtpp_start(C.int(len(argv)), (**C.char)(cvec)); rc != 0 {
        for _, s := range cargvs {
            C.free(unsafe.Pointer(s))
        }
        C.free(cvec)
        cargvs, cvec = nil, nil
        return errors.New("rtpp_main failed to start the engine (see engine log output)")
    }
    state = stateRun
    return nil
}

// Stop shuts the engine down and frees the retained C argv. No-op (nil) if
// never started or already stopped.
func Stop() error {
    mu.Lock()
    defer mu.Unlock()
    if state != stateRun {
        return nil
    }
    if rc := C.rtpp_stop(); rc != 0 {
        // Engine is still up: keep the retained argv (no UAF), report error.
        return errors.New("rtpp_shutdown failed (argv retained)")
    }
    for _, s := range cargvs {
        C.free(unsafe.Pointer(s))
    }
    C.free(cvec)
    cargvs, cvec = nil, nil
    state = stateStopped
    return nil
}
