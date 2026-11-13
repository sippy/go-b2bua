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
// Factory composing the existing UDP SIP transport with a single WSS SIP
// listener (RFC 7118), so that one transaction manager can serve both
// plain UDP peers and browser WebSocket clients.
package sippy

import (
    "sync"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/net"
)

type WebSocketTransportFactory struct {
    config  sippy_conf.Config
    udp     *default_sip_transport_factory
    wss_laddress *sippy_net.HostPort
    wss_opts_template *WssServerOpts // cert/key/sizes; callback filled per-transport
    wss       *WssServer
    lock      sync.Mutex
}

// NewWebSocketTransportFactory returns a sippy_net.SipTransportFactory that
// serves plain UDP for regular SIP addresses and a single WSS listener for
// wss_laddress. Exactly one WSS listener is ever created per factory
// (no duplicate listeners). certFile/keyFile empty = plaintext WS.
func NewWebSocketTransportFactory(config sippy_conf.Config, wss_laddress *sippy_net.HostPort, certFile, keyFile string) *WebSocketTransportFactory {
    return NewWebSocketTransportFactoryWithOpts(config, wss_laddress, &WssServerOpts{
        LAddress : wss_laddress,
        CertFile : certFile,
        KeyFile  : keyFile,
    })
}

// NewWebSocketTransportFactoryWithOpts is like NewWebSocketTransportFactory
// with full control over WSS options (max message size, keepalive,
// subprotocol). The DataCallback field of wss is ignored; local4remote
// installs the transaction manager's handler.
func NewWebSocketTransportFactoryWithOpts(config sippy_conf.Config, wss_laddress *sippy_net.HostPort, wss *WssServerOpts) *WebSocketTransportFactory {
    return &WebSocketTransportFactory{
        config            : config,
        udp               : NewDefaultSipTransportFactory(config),
        wss_laddress      : wss_laddress,
        wss_opts_template : wss,
    }
}

func (self *WebSocketTransportFactory) wssAddressMatches(laddress *sippy_net.HostPort) bool {
    if self.wss_laddress == nil || laddress == nil {
        return false
    }
    return self.wss_laddress.Host.String() == laddress.Host.String() &&
        self.wss_laddress.Port.String() == laddress.Port.String()
}

// NewSipTransport implements sippy_net.SipTransportFactory. Requests for
// the configured WSS address return the single shared WssServer instance;
// everything else is handed to the default UDP factory.
func (self *WebSocketTransportFactory) NewSipTransport(laddress *sippy_net.HostPort, handler sippy_net.DataPacketReceiver) (sippy_net.Transport, error) {
    if self.wssAddressMatches(laddress) {
        return self.connectionTransport(handler)
    }
    return self.udp.NewSipTransport(laddress, handler)
}

// connectionTransport lazily creates (once) and returns the WSS server for
// the given incoming callback. Called by local4remote and by NewSipTransport.
func (self *WebSocketTransportFactory) connectionTransport(handler sippy_net.DataPacketReceiver) (*WssServer, error) {
    self.lock.Lock()
    defer self.lock.Unlock()
    if self.wss == nil {
        opts := *self.wss_opts_template
        opts.DataCallback = handler
        srv, err := NewWssServer(self.config, &opts)
        if err != nil {
            return nil, err
        }
        self.wss = srv
    }
    return self.wss, nil
}

// NewConnectionTransport lazily creates (once) and returns the WSS server
// for the given incoming callback. Called by local4remote and by
// NewSipTransport; it implements sippy_net.ConnectionTransportFactory.
func (self *WebSocketTransportFactory) NewConnectionTransport(handler sippy_net.DataPacketReceiver) (sippy_net.Transport, error) {
    srv, err := self.connectionTransport(handler)
    if err != nil {
        return nil, err
    }
    return srv, nil
}

// WssServer returns the WSS server instance once created (nil before the
// transaction manager started it).
func (self *WebSocketTransportFactory) WssServer() *WssServer {
    self.lock.Lock()
    defer self.lock.Unlock()
    return self.wss
}
