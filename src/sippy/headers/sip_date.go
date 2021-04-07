// Copyright (c) 2021 Sippy Software, Inc. All rights reserved.
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
    "time"

    "sippy/net"
)

var _sip_date_name normalName = newNormalName("Date")

type SipDate struct {
    normalName
    str_body    string
    ts          time.Time
    parsed      bool
}

func CreateSipDate(body string) []SipHeader {
    return []SipHeader{
        &SipDate{
            normalName  : _sip_date_name,
            str_body    : body,
            parsed      : false,
        },
    }
}

func (self *SipDate) GetCopy() *SipDate {
    tmp := *self
    return &tmp
}

func (self *SipDate) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipDate) LocalStr(hostport *sippy_net.HostPort, compact bool) string {
    return self.String()
}

func (self *SipDate) String() string {
    return self.Name() + ": " + self.str_body
}

func (self *SipDate) StringBody() string {
    return self.str_body
}

func (self *SipDate) GetTime() (time.Time, error) {
    var err error

    if self.parsed {
        return self.ts, nil
    }
    self.ts, err = time.Parse("Mon, 2 Jan 2006 15:04:05 MST", self.str_body)
    if err == nil {
        self.parsed = true
    }
    return self.ts, err
}
