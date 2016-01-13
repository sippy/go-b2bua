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
    "strconv"

    "sippy/conf"
)

type SipMaxForwards struct {
    normalName
    number  int
}

var _sip_max_forwards_name normalName = newNormalName("Max-Forwards")

func ParseSipMaxForwards(body string) ([]SipHeader, error) {
    number, err := strconv.Atoi(body)
    if err != nil {
        return nil, err
    }
    return []SipHeader{ &SipMaxForwards{
        normalName  : _sip_max_forwards_name,
        number      : number,
    } }, nil
}

func NewSipMaxForwards() *SipMaxForwards {
    return &SipMaxForwards{
        normalName  : _sip_max_forwards_name,
        number      : 70,
    }
}

func (self *SipMaxForwards) Body() string {
    return strconv.Itoa(self.number)
}

func (self *SipMaxForwards) String() string {
    return self.Name() + ": " + self.Body()
}

func (self *SipMaxForwards) LocalStr(*sippy_conf.HostPort, bool) string {
    return self.String()
}

func (self *SipMaxForwards) GetCopy() *SipMaxForwards {
    tmp := *self
    return &tmp
}

func (self *SipMaxForwards) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
