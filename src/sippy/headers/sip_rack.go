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
    "errors"
    "strconv"

    "sippy/net"
    "sippy/utils"
)

type SipRAckBody struct {
    CSeq    int
    RSeq    int
    Method  string
}

type SipRAck struct {
    normalName
    string_body string
    body        *SipRAckBody
}

func NewSipRAck(rseq, cseq int, method string) *SipRAck {
    return &SipRAck{
        normalName  : _sip_rack_name,
        body        : newSipRAckBody(rseq, cseq, method),
    }
}

func newSipRAckBody(rseq, cseq int, method string) *SipRAckBody {
    return &SipRAckBody{
        RSeq        : rseq,
        CSeq        : cseq,
        Method      : method,
    }
}

var _sip_rack_name normalName = newNormalName("RAck")

func CreateSipRAck(body string) []SipHeader {
    return []SipHeader{
        &SipRAck{
            normalName  : _sip_rack_name,
            string_body : body,
        },
    }
}

func (self *SipRAck) parse() error {
    arr := sippy_utils.FieldsN(self.string_body, 3)
    if len(arr) != 3 {
        return errors.New("Malformed RAck field")
    }
    rseq, err := strconv.Atoi(arr[0])
    if err != nil {
        return err
    }
    cseq, err := strconv.Atoi(arr[1])
    if err != nil {
        return err
    }
    self.body = &SipRAckBody{
        CSeq        : cseq,
        RSeq        : rseq,
        Method      : arr[2],
    }
    return nil
}

func (self *SipRAck) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipRAck) GetBody() (*SipRAckBody, error) {
    if self.body == nil {
        if err := self.parse(); err != nil {
            return nil, err
        }
    }
    return self.body, nil
}

func (self *SipRAck) GetCopy() *SipRAck {
    tmp := *self
    if self.body != nil {
        body := *self.body
        tmp.body = &body
    }
    return &tmp
}

func (self *SipRAck) LocalStr(*sippy_net.HostPort, bool) string {
    return self.String()
}

func (self *SipRAck) String() string {
    return self.Name() + ": " + self.StringBody()
}

func (self *SipRAck) StringBody() string {
    if self.body != nil {
        return self.body.String()
    }
    return self.string_body
}

func (self *SipRAckBody) String() string {
    return strconv.Itoa(self.RSeq) + " " + strconv.Itoa(self.CSeq) + " " + self.Method
}
