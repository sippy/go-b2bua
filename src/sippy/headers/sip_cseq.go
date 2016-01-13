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
    "sippy/utils"
)

type SipCSeq struct {
    normalName
    CSeq    int
    Method  string
}

func NewSipCSeq(cseq int, method string) *SipCSeq {
    return &SipCSeq{
        normalName  : _sip_cseq_name,
        CSeq        : cseq,
        Method      : method,
    }
}

var _sip_cseq_name normalName = newNormalName("CSeq")

func ParseSipCSeq(body string) ([]SipHeader, error) {
    arr := sippy_utils.FieldsN(body, 2)
    cseq, err := strconv.Atoi(arr[0])
    if err != nil {
        return nil, err
    }
    self := &SipCSeq{
        normalName  : _sip_cseq_name,
        CSeq        : cseq,
    }
    if len(arr) == 2 {
        self.Method = arr[1]
    }
    return []SipHeader{ self }, nil
}

func (self *SipCSeq) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipCSeq) GetCopy() *SipCSeq {
    tmp := *self
    return &tmp
}

func (self *SipCSeq) LocalStr(*sippy_conf.HostPort, bool) string {
    return self.String()
}

func (self *SipCSeq) Body() string {
    return strconv.Itoa(self.CSeq) + " " + self.Method
}

func (self *SipCSeq) String() string {
    return self.Name() + ": " + self.Body()
}
