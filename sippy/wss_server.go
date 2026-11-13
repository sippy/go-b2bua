//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2018 Sippy Software, Inc. All rights reserved.
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
// Go port of sippy/Wss_server.py: SIP over WebSocket server (RFC 7118).
// Each open WebSocket connection gets a unique connection identifier
// ("<uuid4>.invalid"); incoming messages are delivered through the same
// DataPacketReceiver callback as UDP, with HostPort.Received set to the
// connection identifier so the transaction manager can route all
// subsequent messages of the dialog back to exactly this connection
// (connection-aware routing), even when several connections share the
// same client IP.
package sippy

import (
    "crypto/rand"
    "crypto/tls"
    "encoding/hex"
    "fmt"
    "net"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/utils"
)

const (
    DefaultWssMaxMessageSize    = 256 * 1024
    DefaultWssKeepaliveInterval = 30 * time.Second
    wssSendQueueSize            = 1000
    wssWriteTimeout             = 10 * time.Second
)

type WssServerOpts struct {
    LAddress          *sippy_net.HostPort
    DataCallback      sippy_net.DataPacketReceiver
    CertFile          string
    KeyFile           string
    MaxMessageSize    int64
    KeepaliveInterval time.Duration
    Subprotocol       string
}

func NewWssServerOpts(laddress *sippy_net.HostPort, data_callback sippy_net.DataPacketReceiver) *WssServerOpts {
    return &WssServerOpts{
        LAddress          : laddress,
        DataCallback      : data_callback,
        MaxMessageSize    : DefaultWssMaxMessageSize,
        KeepaliveInterval : DefaultWssKeepaliveInterval,
        Subprotocol       : "sip",
    }
}

type wssConn struct {
    id      string
    ws      *websocket.Conn
    sendq   chan []byte
    closed  chan struct{}
    once    sync.Once
}

func (self *wssConn) enqueue(data []byte) bool {
    select {
    case self.sendq <- data:
        return true
    case <-self.closed:
        return false
    default:
        return false
    }
}

func (self *wssConn) close() {
    self.once.Do(func() {
        close(self.closed)
        self.ws.Close()
    })
}

type WssServer struct {
    config     sippy_conf.Config
    uopts      WssServerOpts
    ln         net.Listener
    http_srv   *http.Server
    upgrader   websocket.Upgrader
    tls_cfg    *tls.Config
    laddr      *sippy_net.HostPort
    conns      map[string]*wssConn
    lock       sync.RWMutex
    shutdown   bool
    shutdown_once sync.Once
}

func newConnectionID() string {
    b := make([]byte, 16)
    rand.Read(b)
    b[6] = (b[6] & 0x0f) | 0x40
    b[8] = (b[8] & 0x3f) | 0x80
    return fmt.Sprintf("%s-%s-%s-%s-%s.invalid",
        hex.EncodeToString(b[0:4]), hex.EncodeToString(b[4:6]),
        hex.EncodeToString(b[6:8]), hex.EncodeToString(b[8:10]),
        hex.EncodeToString(b[10:16]))
}

// NewWssServer starts the SIP-over-WebSocket server on uopts.LAddress.
// When CertFile/KeyFile are set the listener is TLS (WSS); otherwise it is
// plaintext (WS, for testing). The returned server implements
// sippy_net.Transport and sippy_net.WebSocketTransport.
func NewWssServer(config sippy_conf.Config, uopts *WssServerOpts) (*WssServer, error) {
    if uopts.LAddress == nil || uopts.DataCallback == nil {
        return nil, fmt.Errorf("WssServer: LAddress and DataCallback are required")
    }
    if uopts.MaxMessageSize <= 0 {
        uopts.MaxMessageSize = DefaultWssMaxMessageSize
    }
    if uopts.KeepaliveInterval <= 0 {
        uopts.KeepaliveInterval = DefaultWssKeepaliveInterval
    }
    if uopts.Subprotocol == "" {
        uopts.Subprotocol = "sip"
    }

    self := &WssServer{
        config  : config,
        uopts   : *uopts,
        conns   : make(map[string]*wssConn),
    }
    if uopts.CertFile != "" {
        cert, err := tls.LoadX509KeyPair(uopts.CertFile, uopts.KeyFile)
        if err != nil {
            return nil, fmt.Errorf("WssServer: cannot load cert/key: %s", err.Error())
        }
        self.tls_cfg = &tls.Config{
            Certificates : []tls.Certificate{cert},
            MinVersion   : tls.VersionTLS12,
        }
    }
    self.upgrader = websocket.Upgrader{
        Subprotocols      : []string{self.uopts.Subprotocol},
        // Browser SIP clients connect cross-origin; the WSS endpoint is
        // only reachable by anyone who can reach the server, so origin
        // checks are not applied (matches the Python websockets default).
        CheckOrigin       : func(*http.Request) bool { return true },
        ReadBufferSize    : 4096,
        WriteBufferSize   : 4096,
    }

    ln, err := net.Listen("tcp", self.uopts.LAddress.String())
    if err != nil {
        return nil, err
    }
    if self.tls_cfg != nil {
        ln = tls.NewListener(ln, self.tls_cfg)
    }
    self.ln = ln
    host, port, err := net.SplitHostPort(ln.Addr().String())
    if err != nil {
        ln.Close()
        return nil, err
    }
    self.laddr = sippy_net.NewHostPort(host, port)

    mux := http.NewServeMux()
    mux.HandleFunc("/", self.handleUpgrade)
    self.http_srv = &http.Server{Handler: mux}
    go self.http_srv.Serve(ln)
    return self, nil
}

func (self *WssServer) handleUpgrade(w http.ResponseWriter, r *http.Request) {
    if self.shutdown {
        http.Error(w, "shutting down", http.StatusServiceUnavailable)
        return
    }
    ws, err := self.upgrader.Upgrade(w, r, nil)
    if err != nil {
        // Upgrade failure (bad handshake, unsupported subprotocol, ...):
        // nothing more to do, the response was already sent.
        return
    }
    remote := ws.RemoteAddr().String()
    rhost, rport, err := net.SplitHostPort(remote)
    if err != nil {
        ws.Close()
        return
    }
    conn := &wssConn{
        id     : newConnectionID(),
        ws     : ws,
        sendq  : make(chan []byte, wssSendQueueSize),
        closed : make(chan struct{}),
    }
    self.lock.Lock()
    if self.shutdown {
        self.lock.Unlock()
        conn.close()
        return
    }
    self.conns[conn.id] = conn
    self.lock.Unlock()

    ws.SetReadLimit(self.uopts.MaxMessageSize)
    ka := self.uopts.KeepaliveInterval
    ws.SetReadDeadline(time.Now().Add(ka * 3))
    ws.SetPingHandler(func(appData string) error {
        ws.SetReadDeadline(time.Now().Add(ka * 3))
        return ws.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(wssWriteTimeout))
    })
    ws.SetPongHandler(func(appData string) error {
        ws.SetReadDeadline(time.Now().Add(ka * 3))
        return nil
    })

    go self.writeLoop(conn)
    go self.keepaliveLoop(conn)
    self.readLoop(conn, rhost, rport)
}

// readLoop runs on the upgrade goroutine and blocks until the connection
// dies; it then removes the connection so that stale aliases can never be
// used for further sends.
func (self *WssServer) readLoop(conn *wssConn, rhost, rport string) {
    defer func() {
        self.lock.Lock()
        if self.conns[conn.id] == conn {
            delete(self.conns, conn.id)
        }
        self.lock.Unlock()
        conn.close()
    }()
    for {
        mt, data, err := conn.ws.ReadMessage()
        if err != nil {
            return
        }
        conn.ws.SetReadDeadline(time.Now().Add(self.uopts.KeepaliveInterval * 3))
        if mt != websocket.TextMessage {
            // SIP over WebSocket is text-framed (RFC 7118); drop the
            // connection on anything else.
            return
        }
        rtime, err := sippy_time.NewMonoTime()
        if err != nil {
            self.config.ErrorLogger().Errorf("WssServer: cannot create MonoTime: %s", err.Error())
            continue
        }
        msg := make([]byte, len(data))
        copy(msg, data)
        addr := sippy_net.NewHostPortWithReceived(rhost, rport, conn.id)
        sippy_utils.SafeCall(func() {
            self.uopts.DataCallback(msg, addr, self, rtime)
        }, nil, self.config.ErrorLogger())
    }
}

// writeLoop serializes all outbound writes of one connection.
func (self *WssServer) writeLoop(conn *wssConn) {
    for data := range conn.sendq {
        conn.ws.SetWriteDeadline(time.Now().Add(wssWriteTimeout))
        if err := conn.ws.WriteMessage(websocket.TextMessage, data); err != nil {
            conn.close()
            return
        }
    }
}

func (self *WssServer) keepaliveLoop(conn *wssConn) {
    ticker := time.NewTicker(self.uopts.KeepaliveInterval)
    defer ticker.Stop()
    for {
        select {
        case <-conn.closed:
            return
        case <-ticker.C:
            conn.ws.SetWriteDeadline(time.Now().Add(wssWriteTimeout))
            if err := conn.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
                conn.close()
                return
            }
        }
    }
}

// SendTo routes data to the connection identified by hostport.Host (the
// RFC 7118 connection ID). Unknown or closed aliases are dropped with an
// error log: there is deliberately no UDP fallback for stale WS aliases.
func (self *WssServer) SendTo(data []byte, hostport *sippy_net.HostPort) {
    self.SendToWithCb(data, hostport, nil)
}

func (self *WssServer) SendToWithCb(data []byte, hostport *sippy_net.HostPort, on_complete func()) {
    host := hostport.Host.String()
    self.lock.RLock()
    conn, ok := self.conns[host]
    self.lock.RUnlock()
    if ! ok {
        self.config.ErrorLogger().Errorf("WssServer: no open connection for alias %s, dropping %d bytes (no UDP fallback)", host, len(data))
        if on_complete != nil {
            on_complete()
        }
        return
    }
    if ! conn.enqueue(data) {
        self.config.ErrorLogger().Errorf("WssServer: send queue full or connection closing for alias %s, dropping %d bytes", host, len(data))
    }
    if on_complete != nil {
        on_complete()
    }
}

func (self *WssServer) Shutdown() {
    self.shutdown_once.Do(func() {
        self.lock.Lock()
        self.shutdown = true
        conns := make([]*wssConn, 0, len(self.conns))
        for _, c := range self.conns {
            conns = append(conns, c)
        }
        self.conns = make(map[string]*wssConn)
        self.lock.Unlock()
        for _, c := range conns {
            c.close()
        }
        self.ln.Close()
        self.http_srv.Close()
    })
}

func (self *WssServer) GetLAddress() *sippy_net.HostPort {
    return self.laddr
}

// WssProtocol implements sippy_net.WebSocketTransport: "WSS" when the
// listener is TLS-secured, "WS" for plaintext.
func (self *WssServer) WssProtocol() string {
    if self.tls_cfg != nil {
        return "WSS"
    }
    return "WS"
}

// HasConnection reports whether the RFC 7118 connection alias is currently
// open.
func (self *WssServer) HasConnection(alias string) bool {
    self.lock.RLock()
    _, ok := self.conns[alias]
    self.lock.RUnlock()
    return ok
}

// ConnectionCount returns the number of open WebSocket connections.
func (self *WssServer) ConnectionCount() int {
    self.lock.RLock()
    n := len(self.conns)
    self.lock.RUnlock()
    return n
}
