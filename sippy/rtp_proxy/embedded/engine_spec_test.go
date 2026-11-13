//
// Copyright (c) 2026 Sippy Software, Inc. All rights reserved.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted under the following conditions:
//
// 1. Redistributions of source code must retain the above copyright notice,
//    this list of conditions and the following disclaimer.
//
// 2. Redistributions in source code must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ON AN "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE IMPLIED
// WARRANTIES OF MERCHANTABILITY OR FITNESS FOR A PARTICULAR PURPOSE.
//
// IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY
// DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF USE, DATA OR PROFITS; LOSS
// OF BUSINESS) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OR PERFORMANCE OF THIS SOFTWARE, EVEN IF
// ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//

package embedded

import (
    "encoding/json"
    "os"
    "reflect"
    "testing"
)

// testVector is one rtp_proxy_clients address and the interpretation the
// Python B2BUA reference produces for it (captured by running the actual
// reference code: sippy/Rtp_proxy/client.py __init_internal +
// sippy/Rtp_proxy/Client/internal.py __init with a recording rtpproxy fake;
// see run/rtpio-20261113/lanes/integration/ST4/evidence/py-ref-tokenization.json).
type testVector struct {
    addr     string
    internal bool
    modules  []string // nil = reference produced no modules kwarg
    args     []string // nil = reference produced no extra_args kwarg
}

var vectors = []testVector{
    // external addresses: not rtp.io, left to the existing external client path
    {addr: "unix:/opt/voiptests/b2bua.sock", internal: false},
    {addr: "udp:127.0.0.1:5001", internal: false},
    {addr: "rtp.io", internal: false}, // no colon: reference goes to the unix path
    // plain in-process
    {addr: "rtp.io:", internal: true, modules: []string{}, args: []string{}},
    // static modules (first modules= token, value split on '+')
    {addr: "rtp.io:modules=dtls_gw", internal: true, modules: []string{"dtls_gw"}, args: []string{}},
    {addr: "rtp.io:modules=dtls_gw+ice_lite", internal: true, modules: []string{"dtls_gw", "ice_lite"}, args: []string{}},
    // modules + raw engine args (each ";" token = one engine argv element)
    {addr: "rtp.io:modules=dtls_gw+ice_lite;-l;0.0.0.0;-W;30;-T;45",
        internal: true, modules: []string{"dtls_gw", "ice_lite"},
        args: []string{"-l", "0.0.0.0", "-W", "30", "-T", "45"}},
    {addr: "rtp.io:-m;12000;-M;15000", internal: true, modules: []string{}, args: []string{"-m", "12000", "-M", "15000"}},
    // empty modules= value: reference splits '' on '+' -> ['']
    {addr: "rtp.io:modules=", internal: true, modules: []string{""}, args: []string{}},
    // multiple modules= tokens: first wins, ALL are consumed (not engine args)
    {addr: "rtp.io:modules=dtls_gw+ice_lite;modules=foo",
        internal: true, modules: []string{"dtls_gw", "ice_lite"}, args: []string{}},
    // non-module tokens pass through verbatim
    {addr: "rtp.io:foo;bar", internal: true, modules: []string{}, args: []string{"foo", "bar"}},
    // trailing ';' yields empty tokens (reference behavior)
    {addr: "rtp.io:;", internal: true, modules: []string{}, args: []string{"", ""}},
}

func TestParseRtpIOPythonCompat(t *testing.T) {
    for _, v := range vectors {
        internal, spec, err := ParseRtpIO(v.addr)
        if err != nil {
            t.Fatalf("ParseRtpIO(%q) error: %v", v.addr, err)
        }
        if internal != v.internal {
            t.Errorf("ParseRtpIO(%q): internal=%v, want %v", v.addr, internal, v.internal)
            continue
        }
        if !internal {
            if spec != nil {
                t.Errorf("ParseRtpIO(%q): external must return nil spec", v.addr)
            }
            continue
        }
        // the reference yields no modules kwarg == empty list here (both
        // mean "no static modules")
        if len(spec.Modules) != 0 && len(v.modules) == 0 {
            t.Errorf("ParseRtpIO(%q): modules=%v, want none", v.addr, spec.Modules)
        } else if !reflect.DeepEqual(spec.Modules, v.modules) && !(len(spec.Modules) == 0 && len(v.modules) == 0) {
            t.Errorf("ParseRtpIO(%q): modules=%v, want %v", v.addr, spec.Modules, v.modules)
        }
        if !reflect.DeepEqual(spec.ExtraArgs, v.args) && !(len(spec.ExtraArgs) == 0 && len(v.args) == 0) {
            t.Errorf("ParseRtpIO(%q): extra_args=%v, want %v", v.addr, spec.ExtraArgs, v.args)
        }
    }
}

func TestEngineArgvModulesDso(t *testing.T) {
    _, spec, _ := ParseRtpIO("rtp.io:modules=dtls_gw+ice_lite")
    argv := EngineArgv(spec, []string{"unix:/tmp/ctl.sock"}, "unix:/tmp/b2bua.sock", 12000, 15000)
    want := []string{"rtpproxy", "-f", "-F", "-d", "dbug", "-m", "12000", "-M", "15000",
        "-s", "unix:/tmp/ctl.sock", "-n", "unix:/tmp/b2bua.sock", "--dso", "dtls_gw", "--dso", "ice_lite"}
    if !reflect.DeepEqual(argv, want) {
        t.Errorf("EngineArgv: %v, want %v", argv, want)
    }
}

func TestEngineArgvUserPortRangeWins(t *testing.T) {
    // user-provided -m/-M in the rtp.io: tokens: no duplicate injection
    _, spec, _ := ParseRtpIO("rtp.io:-m;12000;-M;15000")
    argv := EngineArgv(spec, []string{"unix:/tmp/ctl.sock"}, "", 0, 0)
    want := []string{"rtpproxy", "-f", "-F", "-d", "dbug", "-s", "unix:/tmp/ctl.sock", "-m", "12000", "-M", "15000"}
    if !reflect.DeepEqual(argv, want) {
        t.Errorf("EngineArgv: %v, want %v", argv, want)
    }
}

func TestEngineArgvPlain(t *testing.T) {
    _, spec, _ := ParseRtpIO("rtp.io:")
    argv := EngineArgv(spec, nil, "", 0, 0)
    want := []string{"rtpproxy", "-f", "-F", "-d", "dbug"}
    if !reflect.DeepEqual(argv, want) {
        t.Errorf("EngineArgv: %v, want %v", argv, want)
    }
}

func TestHasArg(t *testing.T) {
    if !HasArg([]string{"-m", "12000", "-l", "0.0.0.0"}, "-m") {
        t.Error("HasArg missed -m")
    }
    if HasArg([]string{"-m", "12000"}, "-M") {
        t.Error("HasArg false positive (-m != -M)")
    }
    if HasArg(nil, "-l") {
        t.Error("HasArg on nil must be false")
    }
}

// TestParseRtpIOEquivDump writes the Go interpretation of the representative
// vectors as JSON (same shape as the Python reference capture) when
// RTPIO_EQUIV_OUT is set, so the build runner can diff it against
// py-ref-tokenization.json (the actual python-b2bua reference output).
func TestParseRtpIOEquivDump(t *testing.T) {
    out := os.Getenv("RTPIO_EQUIV_OUT")
    if out == "" {
        t.Skip("RTPIO_EQUIV_OUT not set")
    }
    type result struct {
        Internal    bool     `json:"internal"`
        Modules     []string `json:"modules"`
        ExtraArgs   []string `json:"extra_args"`
    }
    data := map[string]*result{}
    for _, v := range vectors {
        internal, spec, err := ParseRtpIO(v.addr)
        if err != nil {
            t.Fatalf("ParseRtpIO(%q): %v", v.addr, err)
        }
        r := &result{Internal: internal}
        if internal {
            r.Modules = spec.Modules
            if r.Modules == nil {
                r.Modules = []string{}
            }
            r.ExtraArgs = spec.ExtraArgs
            if r.ExtraArgs == nil {
                r.ExtraArgs = []string{}
            }
        }
        data[v.addr] = r
    }
    b, err := json.MarshalIndent(data, "", " ")
    if err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(out, b, 0o644); err != nil {
        t.Fatal(err)
    }
}
