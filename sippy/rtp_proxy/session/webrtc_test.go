// Copyright (c) 2024 Sippy Software, Inc. All rights reserved.
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
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
// ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package rtp_proxy_session

import (
	"strings"
	"sync"
	"testing"

	"github.com/sippy/go-b2bua/sippy"
	"github.com/sippy/go-b2bua/sippy/exceptions"
	"github.com/sippy/go-b2bua/sippy/types"
)

// mockRtppClient is a scripted in-memory RtpProxyClient. It captures the
// exact command strings and replies with pre-prepared results. Results are
// delivered asynchronously (separate goroutine), mirroring the real client,
// because Rtp_proxy_session.send_command holds its inflight lock while
// dispatching.
type mockRtppClient struct {
	mu       sync.Mutex
	commands []string
	results  []string
	n        int
}

func (m *mockRtppClient) SendCommand(cmd string, cb func(string)) {
	m.mu.Lock()
	m.commands = append(m.commands, cmd)
	var r string
	if m.n < len(m.results) {
		r = m.results[m.n]
		m.n++
	}
	m.mu.Unlock()
	go cb(r)
}

func (m *mockRtppClient) SBindSupported() bool                    { return true }
func (m *mockRtppClient) IsLocal() bool                           { return true }
func (m *mockRtppClient) TNotSupported() bool                     { return false }
func (m *mockRtppClient) GetProxyAddress() string                 { return "192.0.2.10" }
func (m *mockRtppClient) IsOnline() bool                          { return true }
func (m *mockRtppClient) GoOnline()                               {}
func (m *mockRtppClient) GoOffline()                              {}
func (m *mockRtppClient) GetOpts() sippy_types.RtpProxyClientOpts { return nil }
func (m *mockRtppClient) Start() error                            { return nil }
func (m *mockRtppClient) UpdateActive(a, b, c, d, e int64)        {}

func (m *mockRtppClient) command(i int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i >= len(m.commands) {
		return ""
	}
	return m.commands[i]
}

func (m *mockRtppClient) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.commands)
}

type testSession struct {
	sess *Rtp_proxy_session
	mc   *mockRtppClient
}

func newTestSession(t *testing.T, results []string) *testSession {
	t.Helper()
	mc := &mockRtppClient{results: results}
	sess, err := NewRtp_proxy_session(nil, []sippy_types.RtpProxyClient{mc},
		"test-call-1", "F0M", "T0G", "", "", &sync.Mutex{})
	if err != nil {
		t.Fatalf("NewRtp_proxy_session: %v", err)
	}
	return &testSession{sess: sess, mc: mc}
}

func sdpBody(t *testing.T, sdp string) sippy_types.MsgBody {
	t.Helper()
	return sippy.NewMsgBody(sdp, "application/sdp")
}

// webrtcSdp is a WebRTC (ICE-lite/DTLS) offer. Note the candidate filter:
// Python drops every 192.x address except 192.168.x, and non-UDP candidates.
// So only candidates 7 (10.10.10.10) and 10 (192.168.1.2) survive de-ICE.
const webrtcSdp = `v=0
o=- 100000 1 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0
m=audio 9 UDP/TLS/RTP/SAVP 111
c=IN IP4 0.0.0.0
a=mid:0
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=sendrecv
a=rtcp-mux
a=rtcp-rsize
a=ice-ufrag:1234
a=ice-pwd:5678
a=ice-options:trickle
a=fingerprint:sha-256 AB:CD:EF
a=setup:actpass
a=msid:mycall 1
a=msid-semantic: WMS mycall
a=ssrc:1 msid:mycall 1
a=ssrc:1 cname:user
a=candidate:7 1 UDP 1518287437 10.10.10.10 30000 typ host
a=candidate:8 1 TCP 1518287437 10.10.10.10 9 typ host
a=candidate:9 1 UDP 1518287437 192.1.2.3 30000 typ host
a=candidate:10 1 UDP 1518287437 192.168.1.2 30000 typ host
`

// sipSdp is a plain RTP/AVP offer from the SIP side.
const sipSdp = `v=0
o=- 200000 2 IN IP4 10.0.0.1
s=-
t=0 0
m=audio 5000 RTP/AVP 0 8
c=IN IP4 10.0.0.5
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=sendrecv
a=rtcp:5001 IN IP4 10.0.0.5
`

func runSdpChange(t *testing.T, s *testSession, body sippy_types.MsgBody, callerSide bool) (sippy_types.MsgBody, sippy_types.SipHandlingError) {
	t.Helper()
	got := make(chan sippy_types.MsgBody, 1)
	gex := make(chan sippy_types.SipHandlingError, 1)
	var ex error
	if callerSide {
		ex = s.sess.OnCallerSdpChange(body, func(b sippy_types.MsgBody, e sippy_types.SipHandlingError) {
			got <- b
			gex <- e
		})
	} else {
		ex = s.sess.OnCalleeSdpChange(body, func(b sippy_types.MsgBody, e sippy_types.SipHandlingError) {
			got <- b
			gex <- e
		})
	}
	if ex != nil {
		he, ok := ex.(sippy_types.SipHandlingError)
		if !ok {
			t.Fatalf("expected SipHandlingError, got %T: %v", ex, ex)
		}
		return nil, he
	}
	b := <-got
	e := <-gex
	return b, e
}

// TestWebrtc2SipCallerSide: webrtc2sip, caller is WebRTC. The incoming
// WebRTC SDP is de-DTLS + de-ICE'd into plain RTP/AVP for the SIP callee.
// Expect "U ... && M4:1 A ... && M5:1 A ... && M5:1 C ... && M5:1 C ...".
func TestWebrtc2SipCallerSide(t *testing.T) {
	s := newTestSession(t, []string{"40000 192.0.2.10 && 0 && 0 && 0 && 0"})
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, webrtcSdp)
	out, ex := runSdpChange(t, s, body, true)
	if ex != nil {
		t.Fatalf("OnCallerSdpChange: %v", ex)
	}
	expected := "U test-call-1-0 0.0.0.0 9 F0M && M4:1 A sha-256 AB:CD:EF 1 && M5:1 A 1234 5678 && M5:1 C 7 1 UDP 1518287437 10.10.10.10 30000 typ host && M5:1 C 10 1 UDP 1518287437 192.168.1.2 30000 typ host"
	if cmd := s.mc.command(0); cmd != expected {
		t.Errorf("command mismatch:\n got: %s\nwant: %s", cmd, expected)
	}
	expectedSdp := "v=0\r\n" +
		"o=- 100000 1 IN IP4 192.0.2.1\r\n" +
		"s=-\r\n" +
		"c=IN IP4 192.0.2.10\r\n" +
		"t=0 0\r\n" +
		"m=audio 40000 RTP/AVP 111\r\n" +
		"a=rtcp-rsize\r\n" +
		"a=ssrc:1 cname:user\r\n" +
		"a=sendonly\r\n"
	if str := out.String(); str != expectedSdp {
		t.Errorf("SDP mismatch:\n got: %q\nwant: %q", str, expectedSdp)
	}
}

// TestWebrtc2SipCalleeSide: webrtc2sip, callee is plain SIP. The SIP SDP is
// converted to a WebRTC (ICE-lite + DTLS) SDP by creating new M4:1 S and
// M5:1 S sessions.
func TestWebrtc2SipCalleeSide(t *testing.T) {
	s := newTestSession(t, []string{"45000 192.0.2.10 && active sha-256 AB:CD:EF && 1234 5678 001%20UDP%201518287437%2010.10.10.10%2030000%20typ%20host"})
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, sipSdp)
	out, ex := runSdpChange(t, s, body, false)
	if ex != nil {
		t.Fatalf("OnCalleeSdpChange: %v", ex)
	}
	expected := "U test-call-1-0 10.0.0.5 5000 T0G && M4:1 S && M5:1 S"
	if cmd := s.mc.command(0); cmd != expected {
		t.Errorf("command mismatch:\n got: %s\nwant: %s", cmd, expected)
	}
	expectedSdp := "v=0\r\n" +
		"o=- 200000 2 IN IP4 192.0.2.1\r\n" +
		"s=-\r\n" +
		"c=IN IP4 192.0.2.10\r\n" +
		"t=0 0\r\n" +
		"a=ice-lite\r\n" +
		"m=audio 45000 UDP/TLS/RTP/SAVP 0 8\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n" +
		"a=sendrecv\r\n" +
		"a=setup:active\r\n" +
		"a=fingerprint:sha-256 AB:CD:EF\r\n" +
		"a=rtcp-mux\r\n" +
		"a=ice-ufrag:1234\r\n" +
		"a=ice-pwd:5678\r\n" +
		"a=candidate:1 UDP 1518287437 10.10.10.10 30000 typ host\r\n"
	if str := out.String(); str != expectedSdp {
		t.Errorf("SDP mismatch:\n got: %q\nwant: %q", str, expectedSdp)
	}
}

// TestSip2WebrtcCallerSide: sip2webrtc, caller is plain SIP. The SIP SDP is
// converted to a WebRTC SDP (M4:1 S + M5:1 S). Exercises SetCalleeWebRTC.
func TestSip2WebrtcCallerSide(t *testing.T) {
	s := newTestSession(t, []string{"45000 192.0.2.10 && active sha-256 AB:CD:EF && 1234 5678 001%20UDP%201518287437%2010.10.10.10%2030000%20typ%20host"})
	s.sess.SetCalleeWebRTC(true)
	body := sdpBody(t, sipSdp)
	out, ex := runSdpChange(t, s, body, true)
	if ex != nil {
		t.Fatalf("OnCallerSdpChange: %v", ex)
	}
	// caller side uses from_tag first
	expected := "U test-call-1-0 10.0.0.5 5000 F0M && M4:1 S && M5:1 S"
	if cmd := s.mc.command(0); cmd != expected {
		t.Errorf("command mismatch:\n got: %s\nwant: %s", cmd, expected)
	}
	if !strings.Contains(out.String(), "m=audio 45000 UDP/TLS/RTP/SAVP 0 8") {
		t.Errorf("expected WebRTC transport:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "a=setup:active") || !strings.Contains(out.String(), "a=fingerprint:sha-256 AB:CD:EF") {
		t.Errorf("expected DTLS attrs:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "a=ice-lite") || !strings.Contains(out.String(), "a=ice-ufrag:1234") {
		t.Errorf("expected ICE attrs:\n%s", out.String())
	}
}

// TestOrdinaryRtpUnchanged: without WebRTC flags the U command is plain
// (no && chaining) and the SDP keeps its transport/rtcp.
func TestOrdinaryRtpUnchanged(t *testing.T) {
	s := newTestSession(t, []string{"45000 192.0.2.10"})
	body := sdpBody(t, sipSdp)
	out, ex := runSdpChange(t, s, body, false)
	if ex != nil {
		t.Fatalf("OnCalleeSdpChange: %v", ex)
	}
	expected := "U test-call-1-0 10.0.0.5 5000 T0G"
	if cmd := s.mc.command(0); cmd != expected {
		t.Errorf("command mismatch:\n got: %s\nwant: %s", cmd, expected)
	}
	if !strings.Contains(out.String(), "m=audio 45000 RTP/AVP 0 8") {
		t.Errorf("transport not preserved:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "a=rtcp:5001 IN IP4 10.0.0.5") {
		t.Errorf("ordinary rtcp changed:\n%s", out.String())
	}
}

func assertRtpProxyError(t *testing.T, ex sippy_types.SipHandlingError, wantSubstr string) {
	t.Helper()
	if ex == nil {
		t.Fatalf("expected RtpProxyError, got nil")
	}
	if _, ok := ex.(*sippy_exceptions.RtpProxyError); !ok {
		t.Fatalf("expected *sippy_exceptions.RtpProxyError, got %T", ex)
	}
	if !strings.Contains(ex.Error(), wantSubstr) {
		t.Errorf("error %q does not contain %q", ex.Error(), wantSubstr)
	}
}

func TestDeDtlsMissingSetup(t *testing.T) {
	s := newTestSession(t, nil)
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, strings.Replace(webrtcSdp, "a=setup:actpass\n", "", 1))
	_, ex := runSdpChange(t, s, body, true)
	if ex == nil || !strings.Contains(ex.Error(), "Missing DTLS connection mode parameter") {
		t.Fatalf("expected missing-setup SdpParseError, got %v", ex)
	}
	if s.mc.count() != 0 {
		t.Errorf("no commands expected, got %d", s.mc.count())
	}
}

func TestDeDtlsMissingFingerprint(t *testing.T) {
	s := newTestSession(t, nil)
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, strings.Replace(webrtcSdp, "a=fingerprint:sha-256 AB:CD:EF\n", "", 1))
	_, ex := runSdpChange(t, s, body, true)
	if ex == nil || !strings.Contains(ex.Error(), "Missing DTLS fingerprint parameter") {
		t.Fatalf("expected missing-fingerprint SdpParseError, got %v", ex)
	}
	if s.mc.count() != 0 {
		t.Errorf("no commands expected, got %d", s.mc.count())
	}
}

func TestDeDtlsUnknownSetup(t *testing.T) {
	s := newTestSession(t, nil)
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, strings.Replace(webrtcSdp, "a=setup:actpass", "a=setup:weird", 1))
	_, ex := runSdpChange(t, s, body, true)
	if ex == nil || !strings.Contains(ex.Error(), `Unknown connection mode: "weird"`) {
		t.Fatalf("expected unknown-mode SdpParseError, got %v", ex)
	}
}

func TestDeIceMissingPwd(t *testing.T) {
	s := newTestSession(t, nil)
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, strings.Replace(webrtcSdp, "a=ice-pwd:5678\n", "", 1))
	_, ex := runSdpChange(t, s, body, true)
	if ex == nil || !strings.Contains(ex.Error(), "Missing ICE ice-pwd parameter") {
		t.Fatalf("expected missing-ice-pwd SdpParseError, got %v", ex)
	}
}

func TestDtlsResultMalformed(t *testing.T) {
	// sip2webrtc, caller (SIP) side: otherside (callee) is WebRTC, so
	// M4:1 S runs. Result "active" has no fingerprint -> strict error.
	s := newTestSession(t, []string{"45000 192.0.2.10 && active"})
	s.sess.SetCalleeWebRTC(true)
	body := sdpBody(t, sipSdp)
	_, ex := runSdpChange(t, s, body, true)
	assertRtpProxyError(t, ex, "DTLS")
}

func TestDtlsResultEmpty(t *testing.T) {
	s := newTestSession(t, []string{"45000 192.0.2.10 && "})
	s.sess.SetCalleeWebRTC(true)
	body := sdpBody(t, sipSdp)
	out, ex := runSdpChange(t, s, body, true)
	_ = out
	assertRtpProxyError(t, ex, "DTLS")
}

func TestIceResultTooShort(t *testing.T) {
	s := newTestSession(t, []string{"40000 192.0.2.10 && 0 && ufragonly"})
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, webrtcSdp)
	_, ex := runSdpChange(t, s, body, true)
	assertRtpProxyError(t, ex, "ICE")
}

func TestIceResultEmptyMiddle(t *testing.T) {
	s := newTestSession(t, []string{"40000 192.0.2.10 && && 0 0 0"})
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, webrtcSdp)
	_, ex := runSdpChange(t, s, body, true)
	assertRtpProxyError(t, ex, "deDTLS")
}

func TestDeDtlsResultNonZero(t *testing.T) {
	s := newTestSession(t, []string{"40000 192.0.2.10 && 1 && 0 0 0"})
	s.sess.SetCallerWebRTC(true)
	body := sdpBody(t, webrtcSdp)
	_, ex := runSdpChange(t, s, body, true)
	assertRtpProxyError(t, ex, "deDTLS")
}

// TestUnquote verifies the Python urllib.parse.unquote semantics used for
// ICE candidates: valid %XX decoded, invalid kept, '+' literal.
func TestUnquote(t *testing.T) {
	if got := unquote("100%200%201%20UDP%20x"); got != "100 0 1 UDP x" {
		t.Errorf("unquote: %q", got)
	}
	if got := unquote("a%ZGb"); got != "a%ZGb" {
		t.Errorf("unquote invalid: %q", got)
	}
	if got := unquote("a+b"); got != "a+b" {
		t.Errorf("unquote plus: %q", got)
	}
}

// Repeated attributes must all be stripped on both levels, without prefix matches.
func TestSdpAttrsRmRepeated(t *testing.T) {
	body := sdpBody(t, sipSdp)
	parsed, err := body.GetSdp()
	if err != nil {
		t.Fatal(err)
	}
	sect := parsed.GetSections()[0]
	parsed.SetAHeaders([]string{"candidate:one", "candidate:two", "rtcp-mux"})
	sect.SetAHeaders([]string{"candidate:three", "candidate:four", "rtcp:9", "rtcp-mux"})
	sdpAttrsRm(parsed, sect, []string{"candidate", "rtcp"})
	if got := strings.Join(parsed.GetAHeaders(), ","); got != "rtcp-mux" {
		t.Fatal(got)
	}
	if got := strings.Join(sect.GetAHeaders(), ","); got != "rtcp-mux" {
		t.Fatal(got)
	}
}
