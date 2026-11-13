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
    "strconv"
    "strings"
)

// RtpIOPrefix is the canonical in-process engine selection prefix for
// rtp_proxy_clients entries (and the webrtc_phone -rtpp value).
//
// The selection is Python-B2BUA compatible: an rtp_proxy_clients entry is
// "rtp.io:" followed by an optional ";"-separated token list. The token
//ization mirrors the Python reference exactly (python-b2bua
// sippy/Rtp_proxy/client.py __init_internal: proto, rest = addr.split(':', 1);
// extra_args = rest.split(';'); and sippy/Rtp_proxy/Client/internal.py
// __init__: the first "modules=" token supplies the static module list
// (value split on '+') and every "modules=" token is consumed; the remaining
// tokens are raw engine arguments, each token being one engine argv element).
//
// Examples:
//
//	rtp.io:                             plain in-process engine
//	rtp.io:modules=dtls_gw+ice_lite     + static modules
//	rtp.io:modules=dtls_gw+ice_lite;-l;0.0.0.0;-m;12000;-M;15000
//	                                    + raw engine args (one per ";" token)
const RtpIOPrefix = "rtp.io:"

// EngineSpec is the parsed form of an rtp.io: engine selection.
type EngineSpec struct {
    // Modules are the static module names from the first "modules=" token
    // (value split on '+'); empty if none.
    Modules []string
    // ExtraArgs are the remaining tokens, each one engine argv element.
    ExtraArgs []string
}

// ParseRtpIO interprets a single rtp_proxy_clients entry. It returns
// (true, spec, nil) when the entry selects the in-process rtp.io engine
// ("rtp.io:" prefix, matching the Python reference startswith('rtp.io:')),
// and (false, nil, nil) for external addresses (unix:/path, udp:host:port,
// tcp:host:port, ...), which keep the existing external-client behavior.
func ParseRtpIO(address string) (bool, *EngineSpec, error) {
    if !strings.HasPrefix(address, RtpIOPrefix) {
        return false, nil, nil
    }
    rest := address[len(RtpIOPrefix):]
    spec := &EngineSpec{}
    if rest == "" {
        return true, spec, nil
    }
    tokens := strings.Split(rest, ";")
    for _, tok := range tokens {
        if strings.HasPrefix(tok, "modules=") {
            if spec.Modules == nil {
                // Python: modules = tuple(modules[0][1].split('+')) — only the
                // first "modules=" token is used.
                spec.Modules = strings.Split(tok[len("modules="):], "+")
            }
            continue // Python drops ALL "modules=" tokens from extra_args
        }
        spec.ExtraArgs = append(spec.ExtraArgs, tok)
    }
    return true, spec, nil
}

// HasArg reports whether the engine arg list contains the given option (e.g.
// "-m"), so callers can avoid injecting a duplicate.
func HasArg(args []string, name string) bool {
    for _, a := range args {
        if a == name {
            return true
        }
    }
    return false
}

// EngineArgv builds the full rtpproxy argv for the in-process engine from a
// parsed rtp.io: selection plus the embedding process's sockets/range:
//
//	rtpproxy -f -F -d dbug [-m MIN -M MAX] -s <ctl>... [-n <notify>]
//	<extra args...> [--dso <module> ...]
//
// portMin/portMax of 0 add no -m/-M (the user's own "-m"/"-M" tokens then
// control the range). The static modules are loaded from the engine's static
// module table via "--dso <name>" (the archive must contain them: built with
// srtp2+OpenSSL detected, see README).
func EngineArgv(spec *EngineSpec, ctlSocks []string, notify string, portMin, portMax int) []string {
    argv := BuildArgv(ctlSocks, portMin, portMax)
    if notify != "" {
        argv = append(argv, "-n", notify)
    }
    if spec != nil {
        argv = append(argv, spec.ExtraArgs...)
        for _, m := range spec.Modules {
            argv = append(argv, "--dso", m)
        }
    }
    return argv
}

// BuildArgv builds an rtpproxy-style command line for the in-process engine:
// foreground, verbose logging, the given RTP port range and the given
// control sockets (same specs understood by RtpProxyClientOpts:
// "unix:/path", "udp:host:port", "tcp:host:port", ...). A zero port bound
// adds no -m/-M.
func BuildArgv(socks []string, portMin, portMax int) []string {
    argv := []string{"rtpproxy", "-f", "-F", "-d", "dbug"}
    if portMin > 0 && portMax > portMin {
        argv = append(argv, "-m", strconv.Itoa(portMin), "-M", strconv.Itoa(portMax))
    }
    for _, s := range socks {
        argv = append(argv, "-s", s)
    }
    return argv
}
