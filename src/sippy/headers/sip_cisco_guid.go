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
    "crypto/rand"
    "strconv"

    "sippy/conf"
)

type SipCiscoGUID struct {
    normalName
    body    string
}

var _sip_cisco_guid_name normalName = newNormalName("Cisco-GUID")

func ParseSipCiscoGUID(body string) ([]SipHeader, error) {
    return []SipHeader{ &SipCiscoGUID{
        normalName : _sip_cisco_guid_name,
        body       : body,
    } }, nil
}

func (self *SipCiscoGUID) Body() string {
    return self.body
}

func (self *SipCiscoGUID) String() string {
    return self.Name() + ": " + self.body
}

func (self *SipCiscoGUID) LocalStr(*sippy_conf.HostPort, bool) string {
    return self.String()
}

func (self *SipCiscoGUID) AsH323ConfId() *SipH323ConfId {
    return &SipH323ConfId{
        normalName  : _sip_h323_conf_id_name,
        body : self.body,
    }
}

func (self *SipCiscoGUID) GetCopy() *SipCiscoGUID {
    tmp := *self
    return &tmp
}

func (self *SipCiscoGUID) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func NewSipCiscoGUID() *SipCiscoGUID {
    arr := make([]byte, 16)
    rand.Read(arr)
    s := ""
    for i := 0; i < 4; i++ {
        i2 := i * 4
        x := uint64(arr[i2]) + (uint64(arr[i2 + 1]) << 8) +
                (uint64(arr[i2 + 2]) << 16) + (uint64(arr[i2 + 3]) << 24)
        if i != 0 { s += "-" }
        s += strconv.FormatUint(x, 10)
    }
    return &SipCiscoGUID{
        normalName : _sip_cisco_guid_name,
        body : s,
    }
}
