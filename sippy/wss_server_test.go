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
// RFC 7118 SIP-over-WebSocket transport tests. All listeners are ephemeral
// (port 0 or a port probed free at runtime); no fixed ports are bound.
package sippy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/sippy/go-b2bua/sippy/conf"
	"github.com/sippy/go-b2bua/sippy/headers"
	"github.com/sippy/go-b2bua/sippy/log"
	"github.com/sippy/go-b2bua/sippy/net"
	"github.com/sippy/go-b2bua/sippy/types"
)

// ---------------------------------------------------------------------------
// Test environment
// ---------------------------------------------------------------------------

type wssTestCallMap struct {
	env *wssTestEnv
}

// Answers every request (REGISTER etc.) with 200 OK.
func (self *wssTestCallMap) OnNewDialog(req sippy_types.SipRequest, tr sippy_types.ServerTransaction) (sippy_types.UA, sippy_types.RequestReceiver, sippy_types.SipResponse) {
	return nil, nil, req.GenResponse(200, "OK", nil, nil)
}

// wssTestUAC is a minimal sippy_types.ResponseReceiver for
// server-originated transactions.
type wssTestUAC struct {
	env    *wssTestEnv
	inv    sippy_types.SipRequest
	got200 chan sippy_types.SipResponse
	calls  atomic.Int32
}

func (self *wssTestUAC) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) {
	// First 200 OK is for the INVITE; later ones (BYE acks) are ignored.
	if self.calls.Add(1) != 1 {
		return
	}
	self.got200 <- resp
}

type wssTestEnv struct {
	config  sippy_conf.Config
	tm      *sipTransactionManager
	factory *WebSocketTransportFactory
	wss     *WssServer
	lock    sync.Mutex
}

func newWssEnv(t *testing.T, wssOpts *WssServerOpts, sipPort string) *wssTestEnv {
	t.Helper()
	config := sippy_conf.NewConfig(sippy_log.NewErrorLogger(), NewTestSipLogger())
	config.SetSipAddress(sippy_net.NewMyAddress("127.0.0.1"))
	config.SetSipPort(sippy_net.NewMyPort(sipPort))
	config.SetIPV6Enabled(false)
	if wssOpts == nil {
		wssOpts = NewWssServerOpts(sippy_net.NewHostPort("127.0.0.1", "0"), nil)
	}
	factory := NewWebSocketTransportFactoryWithOpts(config, sippy_net.NewHostPort("127.0.0.1", "0"), wssOpts)
	config.SetSipTransportFactory(factory)
	env := &wssTestEnv{config: config, factory: factory}
	tm, err := NewSipTransactionManager(config, &wssTestCallMap{env: env})
	if err != nil {
		t.Fatalf("cannot create SIP transaction manager: %s", err.Error())
	}
	env.tm = tm
	env.wss = factory.WssServer()
	if env.wss == nil {
		t.Fatal("WSS server was not created by the factory")
	}
	go tm.Run()
	t.Cleanup(func() { tm.Shutdown() })
	return env
}

func (self *wssTestEnv) wssURL() string {
	return "ws://127.0.0.1:" + self.wss.GetLAddress().Port.String()
}

func dialWss(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	d := websocket.Dialer{Subprotocols: []string{"sip"}, HandshakeTimeout: 5 * time.Second}
	ws, _, err := d.Dial(url, nil)
	if err != nil {
		t.Fatalf("cannot dial %s: %s", url, err.Error())
	}
	return ws
}

func ephemeralUdpPort(t *testing.T) string {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err.Error())
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	c.Close()
	return strconv.Itoa(port)
}

var connIDRe = regexp.MustCompile(`received=([0-9a-f-]+\.invalid)`)

// sendRegister sends a REGISTER and returns the 200 OK text plus the
// connection id picked up from the response Via.
func sendRegister(t *testing.T, ws *websocket.Conn, callID string) (string, string) {
	t.Helper()
	reg := strings.Join([]string{
		"REGISTER sip:127.0.0.1 SIP/2.0",
		fmt.Sprintf("Via: SIP/2.0/WS browser.example.com:56789;branch=z9hG4bK-%s", callID),
		fmt.Sprintf("From: \"Alice\" <sip:alice@example.com>;tag=%s-tag", callID),
		"To: <sip:alice@example.com>",
		fmt.Sprintf("Call-ID: %s", callID),
		"CSeq: 1 REGISTER",
		"Contact: <sip:alice@browser.example.com>",
		"Max-Forwards: 70",
		"Content-Length: 0",
		"",
	}, "\r\n")
	if err := ws.WriteMessage(websocket.TextMessage, []byte(reg)); err != nil {
		t.Fatalf("cannot send REGISTER: %s", err.Error())
	}
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("cannot read 200 OK for REGISTER: %s", err.Error())
	}
	resp := string(data)
	if !strings.Contains(strings.SplitN(resp, "\r\n", 2)[0], "200 OK") {
		t.Fatalf("expected 200 OK, got: %s", resp)
	}
	m := connIDRe.FindStringSubmatch(resp)
	if m == nil {
		t.Fatalf("response Via has no RFC 7118 received= connection id:\n%s", resp)
	}
	return resp, m[1]
}

// gen200OKFor builds a 200 OK for a received request message (echoing the
// dialog headers, adding a To tag and an optional Contact).
func gen200OKFor(reqText string, contact string) string {
	var hdrs []string
	for _, line := range strings.Split(strings.TrimRight(reqText, "\r\n"), "\r\n") {
		up := strings.ToUpper(line)
		if line == "" || strings.HasPrefix(up, "SIP/2.0") {
			continue
		}
		if strings.HasPrefix(up, "VIA:") || strings.HasPrefix(up, "FROM:") ||
			strings.HasPrefix(up, "TO:") || strings.HasPrefix(up, "CALL-ID:") ||
			strings.HasPrefix(up, "CSEQ:") {
			if strings.HasPrefix(up, "TO:") && !strings.Contains(line, "tag=") {
				line += ";tag=browsertag-1"
			}
			hdrs = append(hdrs, line)
		}
	}
	if contact != "" {
		hdrs = append(hdrs, "Contact: <"+contact+">")
	}
	hdrs = append(hdrs, "Content-Length: 0", "")
	return "SIP/2.0 200 OK\r\n" + strings.Join(hdrs, "\r\n")
}

func readWS(t *testing.T, ws *websocket.Conn, what string) string {
	t.Helper()
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("cannot read %s: %s", what, err.Error())
	}
	return string(data)
}

func topViaLine(t *testing.T, msg string) string {
	t.Helper()
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(strings.ToUpper(line), "VIA:") {
			return line
		}
	}
	t.Fatalf("no Via header in message:\n%s", msg)
	return ""
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestWssRegisterRfc7118Routing(t *testing.T) {
	env := newWssEnv(t, nil, "0")
	ws := dialWss(t, env.wssURL())
	defer ws.Close()

	resp, connID := sendRegister(t, ws, "call-reg-1")

	if !strings.Contains(resp, "received="+connID) {
		t.Fatalf("200 OK Via does not carry received=%s:\n%s", connID, resp)
	}
	if !strings.HasSuffix(connID, ".invalid") {
		t.Fatalf("connection id must end with .invalid (RFC 7118): %s", connID)
	}
	if env.wss.ConnectionCount() != 1 {
		t.Fatalf("expected 1 open connection, got %d", env.wss.ConnectionCount())
	}
}

func TestWssDistinctConnectionsSameIP(t *testing.T) {
	env := newWssEnv(t, nil, "0")
	ws1 := dialWss(t, env.wssURL())
	ws2 := dialWss(t, env.wssURL())
	defer ws1.Close()
	defer ws2.Close()

	_, connID1 := sendRegister(t, ws1, "call-dup-1")
	_, connID2 := sendRegister(t, ws2, "call-dup-2")

	if connID1 == connID2 {
		t.Fatalf("two connections from the same IP got the same connection id: %s", connID1)
	}
	// conn1 sends another REGISTER; conn2 must not see it. Only the
	// goroutine writes; the main goroutine owns the read side of ws1.
	go func() {
		time.Sleep(100 * time.Millisecond)
		reg3 := strings.Join([]string{
			"REGISTER sip:127.0.0.1 SIP/2.0",
			"Via: SIP/2.0/WS browser.example.com:56789;branch=z9hG4bK-call-dup-3",
			"From: \"Alice\" <sip:alice@example.com>;tag=call-dup-3-tag",
			"To: <sip:alice@example.com>",
			"Call-ID: call-dup-3",
			"CSeq: 2 REGISTER",
			"Contact: <sip:alice@browser.example.com>",
			"Max-Forwards: 70",
			"Content-Length: 0",
			"",
		}, "\r\n")
		ws1.WriteMessage(websocket.TextMessage, []byte(reg3))
	}()
	got1 := readWS(t, ws1, "200 OK for call-dup-3")
	if !strings.Contains(got1, "Call-ID: call-dup-3") {
		t.Fatalf("conn1 got a foreign response:\n%s", got1)
	}
	ws2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, data2, err := ws2.ReadMessage()
	if err == nil && strings.Contains(string(data2), "call-dup-3") {
		t.Fatalf("conn2 received conn1's response (cross-connection leak):\n%s", string(data2))
	}
}

func TestWssServerOriginatedCallBothDirections(t *testing.T) {
	env := newWssEnv(t, nil, "0")
	ws := dialWss(t, env.wssURL())
	defer ws.Close()

	_, connID := sendRegister(t, ws, "call-soc-1")

	// Server-originated INVITE targeting the browser's connection alias.
	ruri, err := sippy_header.ParseSipURL("sip:alice@example.com", false, env.config)
	if err != nil {
		t.Fatal(err.Error())
	}
	target := sippy_net.NewHostPort(connID, "5060")
	inv, err := NewSipRequest("INVITE", ruri, "",
		sippy_header.NewSipTo(sippy_header.NewSipAddress("", ruri), env.config),
		sippy_header.NewSipFrom(sippy_header.NewSipAddress("Alice", ruri), env.config),
		nil, 1, nil, nil, nil, nil, nil, target, nil, nil, env.config)
	if err != nil {
		t.Fatal(err.Error())
	}
	uac := &wssTestUAC{env: env, inv: inv, got200: make(chan sippy_types.SipResponse, 1)}
	env.tm.BeginNewClientTransaction(inv, uac, &env.lock, nil, nil, nil)

	// Browser receives the INVITE on the same connection; top Via must be
	// SIP/2.0/WS (plaintext) instead of the default UDP.
	invite := readWS(t, ws, "INVITE")
	if via := topViaLine(t, invite); !strings.HasPrefix(via, "Via: SIP/2.0/WS ") {
		t.Fatalf("server-originated INVITE top Via is not SIP/2.0/WS:\n%s", via)
	}

	// Browser answers 200 OK with its own contact; the TM must rewrite the
	// contact host to the connection id before the UA layer sees it.
	if err := ws.WriteMessage(websocket.TextMessage,
		[]byte(gen200OKFor(invite, "sip:alice@browser.example.com"))); err != nil {
		t.Fatal(err.Error())
	}
	var resp sippy_types.SipResponse
	select {
	case resp = <-uac.got200:
	case <-time.After(5 * time.Second):
		t.Fatal("200 OK was not delivered to the client transaction")
	}
	if len(resp.GetContacts()) != 1 {
		t.Fatalf("200 OK has no Contact, cannot route ACK/BYE")
	}
	cbody, err := resp.GetContacts()[0].GetBody(env.config)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cbody.GetUrl().Host.String() != connID {
		t.Fatalf("response Contact host was not rewritten to the connection id: got %s want %s",
			cbody.GetUrl().Host.String(), connID)
	}

	// Server sends ACK and BYE back over the same connection.
	to := resp.GetTo()
	// The client transaction automatically ACKs the successful INVITE.
	ackText := readWS(t, ws, "ACK")
	if !strings.HasPrefix(ackText, "ACK ") {
		t.Fatalf("expected ACK, got: %s", ackText)
	}
	// Browser acknowledges the ACK's transaction (200 OK for the INVITE is
	// already final; just keep the connection quiet and continue with BYE).
	bye, err := NewSipRequest("BYE", cbody.GetUrl(), "", to, inv.GetFrom(),
		nil, 2, inv.GetCallId(), nil, nil, nil, nil, target, nil, nil, env.config)
	if err != nil {
		t.Fatal(err.Error())
	}
	env.tm.BeginNewClientTransaction(bye, uac, &env.lock, nil, nil, nil)
	byeText := readWS(t, ws, "BYE")
	if !strings.HasPrefix(byeText, "BYE ") {
		t.Fatalf("expected BYE, got: %s", byeText)
	}
	if err := ws.WriteMessage(websocket.TextMessage, []byte(gen200OKFor(byeText, ""))); err != nil {
		t.Fatal(err.Error())
	}

	// BYE in the other direction (browser -> server) is also served over
	// the same connection.
	bye2 := strings.Join([]string{
		"BYE sip:alice@example.com SIP/2.0",
		"Via: SIP/2.0/WS browser.example.com:56789;branch=z9hG4bK-bye2-soc",
		"From: \"Alice\" <sip:alice@example.com>;tag=call-soc-1-tag",
		"To: <sip:alice@example.com>;tag=browsertag-1",
		"Call-ID: call-soc-1",
		"CSeq: 3 BYE",
		"Max-Forwards: 70",
		"Content-Length: 0",
		"",
	}, "\r\n")
	if err := ws.WriteMessage(websocket.TextMessage, []byte(bye2)); err != nil {
		t.Fatal(err.Error())
	}
	ok := readWS(t, ws, "200 OK for browser BYE")
	if !strings.Contains(strings.SplitN(ok, "\r\n", 2)[0], "200 OK") {
		t.Fatalf("browser BYE was not answered over WSS:\n%s", ok)
	}
}

func TestWssStaleAliasNoUdpFallback(t *testing.T) {
	env := newWssEnv(t, nil, "0")
	ws := dialWss(t, env.wssURL())
	_, connID := sendRegister(t, ws, "call-stale-1")
	ws.Close()

	// Wait until the server cleaned up the closed connection.
	deadline := time.Now().Add(3 * time.Second)
	for env.wss.ConnectionCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if env.wss.ConnectionCount() != 0 {
		t.Fatal("closed connection was not cleaned up")
	}

	// A request targeting the stale alias must be dropped: no UDP send,
	// no delivery, no response on any transport.
	ruri, _ := sippy_header.ParseSipURL("sip:alice@example.com", false, env.config)
	inv, err := NewSipRequest("INVITE", ruri, "",
		sippy_header.NewSipTo(sippy_header.NewSipAddress("", ruri), env.config),
		sippy_header.NewSipFrom(sippy_header.NewSipAddress("Alice", ruri), env.config),
		nil, 1, nil, nil, nil, nil, nil,
		sippy_net.NewHostPort(connID, "5060"), nil, nil, env.config)
	if err != nil {
		t.Fatal(err.Error())
	}
	got := make(chan sippy_types.SipResponse, 1)
	uac := &wssTestUAC{env: env, inv: inv, got200: got}
	env.tm.BeginNewClientTransaction(inv, uac, &env.lock, nil, nil, nil)
	select {
	case <-got:
		t.Fatal("a stale WS alias was answered (it must be dropped, not rerouted)")
	case <-time.After(1500 * time.Millisecond):
	}
}

func TestWssOversizedFrameClosesConnection(t *testing.T) {
	env := newWssEnv(t, nil, "0")
	ws := dialWss(t, env.wssURL())
	defer ws.Close()
	sendRegister(t, ws, "call-big-1")

	big := make([]byte, 300*1024)
	for i := range big {
		big[i] = 'a'
	}
	if err := ws.WriteMessage(websocket.TextMessage, big); err != nil {
		t.Fatalf("cannot send oversized frame: %s", err.Error())
	}
	deadline := time.Now().Add(3 * time.Second)
	for env.wss.ConnectionCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if env.wss.ConnectionCount() != 0 {
		t.Fatal("oversized message did not close the connection (bounded messages violated)")
	}
}

func TestWssKeepalivePing(t *testing.T) {
	opts := NewWssServerOpts(sippy_net.NewHostPort("127.0.0.1", "0"), nil)
	opts.KeepaliveInterval = 200 * time.Millisecond
	env := newWssEnv(t, opts, "0")
	ws := dialWss(t, env.wssURL())
	defer ws.Close()

	pingCh := make(chan struct{}, 4)
	ws.SetPingHandler(func(appData string) error {
		pingCh <- struct{}{}
		return nil
	})
	// Gorilla processes control frames in the read pump; keep reading
	// (discarding) so PINGs are surfaced to the handler.
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()
	select {
	case <-pingCh:
	case <-time.After(3 * time.Second):
		t.Fatal("no keepalive PING received from the server")
	}
}

func TestWssUdpCompatibility(t *testing.T) {
	// The same transaction manager serves plain UDP peers alongside the
	// WSS listener; a UDP REGISTER must still get a 200 OK over UDP.
	port := ephemeralUdpPort(t)
	env := newWssEnv(t, nil, port)
	if env.wss.GetLAddress() == nil {
		t.Fatal("WSS listener did not start alongside the UDP transport")
	}

	// Dial directly to the TM's UDP listen port.
	udpDest := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: mustAint(t, port)}
	conn, err := net.DialUDP("udp", nil, udpDest)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer conn.Close()
	lp := conn.LocalAddr().(*net.UDPAddr).Port
	reg := strings.Join([]string{
		"REGISTER sip:127.0.0.1 SIP/2.0",
		fmt.Sprintf("Via: SIP/2.0/UDP 127.0.0.1:%d;branch=z9hG4bK-udp-1", lp),
		"From: \"Bob\" <sip:bob@example.com>;tag=bobtag-1",
		"To: <sip:bob@example.com>",
		"Call-ID: call-udp-1@example.com",
		"CSeq: 1 REGISTER",
		"Contact: <sip:bob@peer.example.com>",
		"Max-Forwards: 70",
		"Content-Length: 0",
		"",
	}, "\r\n")
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(reg)); err != nil {
		t.Fatal(err.Error())
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("no 200 OK over UDP: %s", err.Error())
	}
	resp := string(buf[:n])
	if !strings.Contains(strings.SplitN(resp, "\r\n", 2)[0], "200 OK") {
		t.Fatalf("UDP REGISTER was not answered with 200 OK:\n%s", resp)
	}
	if !strings.Contains(resp, "Call-ID: call-udp-1@example.com") {
		t.Fatalf("UDP response is for the wrong dialog:\n%s", resp)
	}
}

func TestWssTls(t *testing.T) {
	certFile, keyFile := genSelfSignedCert(t)
	opts := NewWssServerOpts(sippy_net.NewHostPort("127.0.0.1", "0"), nil)
	opts.CertFile = certFile
	opts.KeyFile = keyFile
	env := newWssEnv(t, opts, "0")
	if env.wss.WssProtocol() != "WSS" {
		t.Fatalf("TLS server must report WSS, got %s", env.wss.WssProtocol())
	}

	d := websocket.Dialer{
		Subprotocols:     []string{"sip"},
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		HandshakeTimeout: 5 * time.Second,
	}
	u, _ := url.Parse("wss://127.0.0.1:" + env.wss.GetLAddress().Port.String())
	ws, _, err := d.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("cannot dial TLS WSS: %s", err.Error())
	}
	defer ws.Close()

	resp, connID := sendRegister(t, ws, "call-tls-1")
	if !strings.Contains(resp, "received="+connID) {
		t.Fatalf("TLS 200 OK has no connection id:\n%s", resp)
	}

	// Server-originated request over TLS must carry SIP/2.0/WSS.
	ruri, _ := sippy_header.ParseSipURL("sip:alice@example.com", false, env.config)
	inv, err := NewSipRequest("INVITE", ruri, "",
		sippy_header.NewSipTo(sippy_header.NewSipAddress("", ruri), env.config),
		sippy_header.NewSipFrom(sippy_header.NewSipAddress("Alice", ruri), env.config),
		nil, 1, nil, nil, nil, nil, nil,
		sippy_net.NewHostPort(connID, "5060"), nil, nil, env.config)
	if err != nil {
		t.Fatal(err.Error())
	}
	uac := &wssTestUAC{env: env, inv: inv, got200: make(chan sippy_types.SipResponse, 1)}
	env.tm.BeginNewClientTransaction(inv, uac, &env.lock, nil, nil, nil)
	invite := readWS(t, ws, "INVITE")
	if via := topViaLine(t, invite); !strings.HasPrefix(via, "Via: SIP/2.0/WSS ") {
		t.Fatalf("TLS server-originated INVITE top Via is not SIP/2.0/WSS:\n%s", via)
	}
}

func mustAint(t *testing.T, s string) int {
	t.Helper()
	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err.Error())
	}
	return v
}

func genSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err.Error())
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err.Error())
	}
	dir := t.TempDir()
	certFile := dir + "/cert.pem"
	keyFile := dir + "/key.pem"
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err.Error())
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err.Error())
	}
	return certFile, keyFile
}
