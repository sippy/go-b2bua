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

type SipReferTo struct {
    compactName
    *sipAddressHF
}

var _sip_refer_to_name compactName = newCompactName("Refer-To", "r")

func CreateSipReferTo(body string) []SipHeader {
    addresses := createSipAddressHFs(body)
    rval := make([]SipHeader, len(addresses))
    for i, addr := range addresses {
        rval[i] = &SipReferTo{
            compactName  : _sip_refer_to_name,
            sipAddressHF : addr,
        }
    }
    return rval
}

func NewSipReferTo(addr *sipAddress) *SipReferTo {
    return &SipReferTo{
        compactName  : _sip_refer_to_name,
        sipAddressHF : newSipAddressHF(addr),
    }
}

func (self *SipReferTo) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipReferTo) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    prefix := self.Name()
    if compact {
        prefix = self.CompactName()
    }
    return prefix + ": " + self.LocalStringBody(hostport)
}

func (self *SipReferTo) AsSipAlso() *SipAlso {
    return &SipAlso{
        normalName   : _sip_also_name,
        sipAddressHF : self.sipAddressHF.getCopy(),
    }
}

func (self *SipReferTo) GetCopy() *SipReferTo {
    return &SipReferTo{
        compactName  : _sip_refer_to_name,
        sipAddressHF : self.sipAddressHF.getCopy(),
    }
}

func (self *SipReferTo) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
