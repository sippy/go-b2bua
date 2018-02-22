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
    "sippy/net"
)

type SipContentType struct {
    compactName
    body string
}

var _sip_content_type_name compactName = newCompactName("Content-Type", "c")

func CreateSipContentType(body string) []SipHeader {
    return []SipHeader{
        &SipContentType{
            compactName : _sip_content_type_name,
            body        : body,
        },
    }
}

func (self *SipContentType) StringBody() string {
    return self.body
}

func (self *SipContentType) String() string {
    return self.Name() + ": " + self.body
}

func (self *SipContentType) LocalStr(hostport *sippy_net.HostPort, compact bool) string {
    if compact {
        return self.CompactName() + ": " + self.body
    }
    return self.String()
}

func (self *SipContentType) GetCopy() *SipContentType {
    tmp := *self
    return &tmp
}

func (self *SipContentType) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
