// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2019 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2019 Andrii Pylypenko. All rights reserved.
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

type SipRSeq struct {
    normalName
    SipNumericHF
}

var _sip_rseq_name normalName = newNormalName("RSeq")

func NewSipRSeq() *SipRSeq {
    return &SipRSeq{
        normalName  : _sip_rseq_name,
        SipNumericHF : newSipNumericHF(1),
    }
}

func CreateSipRSeq(body string) []SipHeader {
    return []SipHeader{ &SipRSeq{
        normalName      : _sip_rseq_name,
        SipNumericHF    : createSipNumericHF(body),
    } }
}

func (self *SipRSeq) String() string {
    return self.Name() + ": " + self.StringBody()
}

func (self *SipRSeq) LocalStr(hostport *sippy_net.HostPort, compact bool) string {
    return self.String()
}

func (self *SipRSeq) GetCopy() *SipRSeq {
    tmp := *self
    return &tmp
}

func (self *SipRSeq) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipRSeq) IncNum() {
    self.parse()
    self.Number++
}
