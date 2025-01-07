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

package sippy_exceptions

import (
    "strconv"

    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/headers"
)

type SipParseError struct {
    code  int
    scode string
    msg   string
}

func NewSipParseError(msg string) *SipParseError {
    return &SipParseError{msg: msg, code: 400, scode: "Bad Request - " + msg}
}

func (self *SipParseError) Error() string {
    return self.msg
}

func (self *SipParseError) GetResponse(req sippy_types.SipRequest) sippy_types.SipResponse {
    resp := req.GenResponse(self.code, self.scode, nil, nil)
    if reason := self.GetReason(); reason != nil {
        resp.AppendHeader(reason)
    }
    return resp
}

func (self *SipParseError) GetReason() *sippy_header.SipReason {
    if self.msg != "" {
        return sippy_header.NewSipReason("SIP", strconv.Itoa(self.code), self.msg)
    }
    return nil
}

func (self *SipParseError) GetEvent(ctor sippy_types.GetEventCtor) sippy_types.CCEvent {
    return ctor(self.code, self.scode, self.GetReason())
}
