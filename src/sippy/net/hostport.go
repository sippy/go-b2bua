//
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

package sippy_net

import (
    "net"
    "strings"
)

type MyAddress struct {
    is_system   bool
    address     string
}

type MyPort struct {
    is_system   bool
    port        string
}

/* MyAddress methods */

func NewMyAddress(address string) (*MyAddress) {
    self := &MyAddress{
        is_system   : false,
        address     : address,
    }
    self.normalize()
    return self
}

func NewSystemAddress(address string) (*MyAddress) {
    self := &MyAddress{
        is_system   : true,
        address     : address,
    }
    self.normalize()
    return self
}

func (self *MyAddress) normalize() {
    if self.address == "" {
        return
    }
    if self.address[0] != '[' && (strings.IndexByte(self.address, ':') >= 0 || strings.IndexByte(self.address, '%') >= 0) {
        self.address = "[" + self.address + "]"
    }
}

func (self *MyAddress) IsSystemDefault() bool {
    return self.is_system
}

func (self *MyAddress) ParseIP() net.IP {
    if self.address[0] == '[' {
        return net.ParseIP(self.address[1:len(self.address)-1])
    }
    return net.ParseIP(self.address)
}

func (self *MyAddress) String() string {
    return self.address
}

func (self *MyAddress) GetCopy() *MyAddress {
    tmp := *self
    return &tmp
}

/* MyPort methods */

func NewMyPort(port string) (*MyPort) {
    return &MyPort{
        is_system   : false,
        port        : port,
    }
}

func NewSystemPort(port string) (*MyPort) {
    return &MyPort{
        is_system   : true,
        port        : port,
    }
}

func (self *MyPort) String() string {
    return self.port
}

func (self *MyPort) IsSystemDefault() bool {
    return self.is_system
}

func (self *MyPort) GetCopy() *MyPort {
    tmp := *self
    return &tmp
}

/* HostPort */
type HostPort struct {
    Host    *MyAddress
    Port    *MyPort
}

func NewHostPort(host, port string) *HostPort {
    return &HostPort{
        Host : NewMyAddress(host),
        Port : NewMyPort(port),
    }
}

func NewHostPortFromAddr(addr net.Addr) (*HostPort, error) {
    host, port, err := net.SplitHostPort(addr.String())
    if err != nil {
        return nil, err
    }
    return NewHostPort(host, port), nil
}

func (self *HostPort) ParseIP() net.IP {
    return self.Host.ParseIP()
}

func (self *HostPort) String() string {
    if self == nil {
        return "nil"
    }
    return self.Host.String() + ":" + self.Port.String()
}

func (self *HostPort) GetCopy() *HostPort {
    return &HostPort{
        Host : self.Host.GetCopy(),
        Port : self.Port.GetCopy(),
    }
}
