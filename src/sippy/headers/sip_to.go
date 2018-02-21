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

type SipTo struct {
    compactName
    *sipAddressHF
}

var _sip_to_name compactName = newCompactName("To", "t")

func NewSipTo(address *SipAddress, config sippy_conf.Config) *SipTo {
    if address == nil {
        address = NewSipAddress("Anonymous", NewSipURL("" /* username */,
                                    config.GetMyAddress(),
                                    config.GetMyPort(),
                                    false))
    }
    return &SipTo{
        compactName     : _sip_to_name,
        sipAddressHF    : newSipAddressHF(address, config),
    }
}

func CreateSipTo(body string) []SipHeader {
    addresses := createSipAddressHFs(body)
    rval := make([]SipHeader, len(addresses))
    for i, addr := range addresses {
        rval[i] = &SipTo{
            compactName : _sip_to_name,
            sipAddressHF : addr,
        }
    }
    return rval
}

func (self *SipTo) GetCopy() *SipTo {
    return &SipTo{
        compactName : _sip_to_name,
        sipAddressHF : self.sipAddressHF.getCopy(),
    }
}

func (self *SipTo) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipTo) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipTo) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    if compact {
        return self.CompactName() + ": " + self.LocalStringBody(hostport)
    }
    return self.Name() + ": " + self.LocalStringBody(hostport)
}
