// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2015 Andrii Pylypenko. All rights reserved.
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
package sippy

import (
    "net"
    "sync"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/net"
)

type local4remote struct {
    config          sippy_conf.Config
    cache_r2l       map[string]*sippy_net.HostPort
    cache_r2l_old   map[string]*sippy_net.HostPort
    cache_l2s       map[string]sippy_net.Transport
    handleIncoming  sippy_net.DataPacketReceiver
    fixed           bool
    tfactory        sippy_net.SipTransportFactory
    lock            sync.Mutex
}

func NewLocal4Remote(config sippy_conf.Config, handleIncoming sippy_net.DataPacketReceiver) (*local4remote, error) {
    self := &local4remote{
        config          : config,
        cache_r2l       : make(map[string]*sippy_net.HostPort),
        cache_r2l_old   : make(map[string]*sippy_net.HostPort),
        cache_l2s       : make(map[string]sippy_net.Transport),
        handleIncoming  : handleIncoming,
        fixed           : false,
        tfactory        : config.GetSipTransportFactory(),
    }
    if self.tfactory == nil {
        self.tfactory = NewDefaultSipTransportFactory(config)
    }
    laddresses := make([]*sippy_net.HostPort, 0)
    if config.SipAddress().IsSystemDefault() {
        laddresses = append(laddresses, sippy_net.NewHostPort("0.0.0.0", config.SipPort().String()))
        if config.GetIPV6Enabled() {
            laddresses = append(laddresses, sippy_net.NewHostPort("[::]", config.SipPort().String()))
        }
    } else {
        laddresses = append(laddresses, sippy_net.NewHostPort(config.SipAddress().String(), config.SipPort().String()))
        self.fixed = true
    }
    var last_error error
    for _, laddress := range laddresses {
        /*
        sopts := NewUdpServerOpts(laddress, handleIncoming)
        server, err := NewUdpServer(config, sopts)
        */
        server, err := self.tfactory.NewSipTransport(laddress, handleIncoming)
        if err != nil {
            if ! config.SipAddress().IsSystemDefault() {
                return nil, err
            } else {
                last_error = err
            }
        } else {
            self.cache_l2s[laddress.String()] = server
        }
    }
    if len(self.cache_l2s) == 0 && last_error != nil {
        return nil, last_error
    }
    return self, nil
}

func (self *local4remote) getServer(address *sippy_net.HostPort, is_local bool /*= false*/) sippy_net.Transport {
    var laddress *sippy_net.HostPort
    var ok bool

    self.lock.Lock()
    defer self.lock.Unlock()

    if self.fixed {
        for _, server := range self.cache_l2s {
            return server
        }
        return nil
    }
    if ! is_local {
        laddress, ok = self.cache_r2l[address.Host.String()]
        if ! ok {
            laddress, ok = self.cache_r2l_old[address.Host.String()]
            if ok {
                self.cache_r2l[address.Host.String()] = laddress
            }
        }
        if ok {
            server, ok := self.cache_l2s[laddress.String()]
            if ! ok {
                return nil
            } else {
                //print 'local4remote-1: local address for %s is %s' % (address[0], laddress[0])
                return server
            }
        }
        lookup_address, err := net.ResolveUDPAddr("udp", address.String())
        if err != nil {
            return nil
        }
        _laddress := ""
        c, err := net.ListenUDP("udp", lookup_address)
        if err == nil {
            c.Close()
            _laddress, _, err = net.SplitHostPort(lookup_address.String())
            if err != nil {
                return nil // should not happen
            }
        } else {
            conn, err := net.DialUDP("udp", nil, lookup_address)
            if err != nil {
                return nil // should not happen
            }
            _laddress, _, err = net.SplitHostPort(conn.LocalAddr().String())
            conn.Close()
            if err != nil {
                return nil // should not happen
            }
        }
        laddress = sippy_net.NewHostPort(_laddress, self.config.SipPort().String())
        self.cache_r2l[address.Host.String()] = laddress
    } else {
        laddress = address
    }
    server, ok := self.cache_l2s[laddress.String()]
    if ! ok {
        var err error
        /*
        sopts := NewUdpServerOpts(laddress, self.handleIncoming)
        server, err = NewUdpServer(self.config, sopts)
        */
        server, err = self.tfactory.NewSipTransport(laddress, self.handleIncoming)
        if err != nil {
            self.config.ErrorLogger().Errorf("Cannot bind %s: %s", laddress.String(), err.Error())
            return nil
        }
        self.cache_l2s[laddress.String()] = server
    }
    //print 'local4remote-2: local address for %s is %s' % (address[0], laddress[0])
    return server
}

func (self *local4remote) rotateCache() {
    self.lock.Lock()
    defer self.lock.Unlock()

    self.cache_r2l_old = self.cache_r2l
    self.cache_r2l = make(map[string]*sippy_net.HostPort)
}

func (self *local4remote) shutdown() {
    for _, userv := range self.cache_l2s {
        userv.Shutdown()
    }
    self.cache_l2s = make(map[string]sippy_net.Transport)
}

