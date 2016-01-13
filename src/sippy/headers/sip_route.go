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
package sippy_header

import (
    "sippy/conf"
)

type SipRoute struct {
    normalName
    sipAddressHF
}

var _sip_route_name normalName = newNormalName("Route")

func NewSipRoute(addr *sipAddress) *SipRoute {
    return &SipRoute{
        normalName   : _sip_route_name,
        sipAddressHF : *NewSipAddressHF(addr),
    }
}

func ParseSipRoute(body string) ([]SipHeader, error) {
    addresses, err := ParseSipAddressHF(body)
    if err != nil { return nil, err }
    rval := make([]SipHeader, len(addresses))
    for i, addr := range addresses {
        rval[i] = &SipRoute{
            normalName   : _sip_route_name,
            sipAddressHF : *addr,
        }
    }
    return rval, nil
}

func (self *SipRoute) String() string {
    return self.Name() + ": " + self.Address.String()
}

func (self *SipRoute) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    return self.Name() + ": " + self.Address.LocalStr(hostport)
}

func (self *SipRoute) GetAddr(config sippy_conf.Config) *sippy_conf.HostPort {
    return self.Address.url.GetAddr(config)
}

func (self *SipRoute) GetCopy() *SipRoute {
    return &SipRoute{
        normalName   : _sip_route_name,
        sipAddressHF : *self.sipAddressHF.getCopy(),
    }
}

func (self *SipRoute) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
