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
    "github.com/sippy/go-b2bua/sippy/net"
)

type SipH323ConfId struct {
    normalName
    body    string
}

var _sip_h323_conf_id_name normalName = newNormalName("h323-conf-id")

func CreateSipH323ConfId(body string) []SipHeader {
    return []SipHeader{
        &SipH323ConfId{
            normalName  : _sip_h323_conf_id_name,
            body        : body,
        },
    }
}

func (self *SipH323ConfId) GetCopy() *SipH323ConfId {
    tmp := *self
    return &tmp
}

func (self *SipH323ConfId) StringBody() string {
    return self.body
}

func (self *SipH323ConfId) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipH323ConfId) String() string {
    return self.Name() + ": " + self.body
}

func (self *SipH323ConfId) LocalStr(*sippy_net.HostPort, bool) string {
    return self.String()
}

func (self *SipH323ConfId) AsCiscoGUID() *SipCiscoGUID {
    return &SipCiscoGUID{
        normalName  : _sip_cisco_guid_name,
        body : self.body,
    }
}
