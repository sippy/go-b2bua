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
    "errors"
    "strings"

    "sippy/net"
)

type sipReasonBody struct {
    reason  string
    cause   string
    protocol string
}

func newSipReasonBody(protocol, cause, reason string) *sipReasonBody {
    return &sipReasonBody{
        reason      : reason,
        protocol    : protocol,
        cause       : cause,
    }
}

func (self *sipReasonBody) String() string {
    var rval string
    if self.reason == "" {
        rval = self.protocol + "; cause=" + self.cause
    } else {
        rval = self.protocol + "; cause=" + self.cause + "; text=\"" + self.reason + "\""
    }
    return rval
}

type SipReason struct {
    normalName
    string_body     string
    body            *sipReasonBody
}

var _sip_reason_name normalName = newNormalName("Reason")

func CreateSipReason(body string) []SipHeader {
    return []SipHeader{
        &SipReason{
            normalName      : _sip_reason_name,
            string_body     : body,
        },
    }
}

func (self *SipReason) parse() error {
    arr := strings.SplitN(self.string_body, ";", 2)
    if len(arr) != 2 {
        return errors.New("Error parsing Reason: (1)")
    }
    protocol, reason_params := arr[0], arr[1]
    body := &sipReasonBody{
        protocol : strings.TrimSpace(protocol),
    }
    for _, reason_param := range strings.Split(reason_params, ";") {
        arr = strings.SplitN(reason_param, "=", 2)
        if len(arr) != 2 {
            return errors.New("Error parsing Reason: (2)")
        }
        rp_name, rp_value := strings.TrimSpace(arr[0]), strings.TrimSpace(arr[1])
        switch rp_name {
        case "cause":
            body.cause = rp_value
        case "text":
            body.reason = strings.Trim(rp_value, "\"")
        }
    }
    self.body = body
    return nil
}

func (self *SipReason) StringBody() string {
    if self.body != nil {
        return self.body.String()
    }
    return self.string_body
}

func (self *SipReason) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipReason) LocalStr(hostport *sippy_net.HostPort, compact bool) string {
    return self.Name() + ": " + self.StringBody()
}

func NewSipReason(protocol, cause, reason string) *SipReason {
    return &SipReason{
        normalName  : _sip_reason_name,
        body        : newSipReasonBody(protocol, cause, reason),
    }
}

func (self *SipReason) GetCopy() *SipReason {
    tmp := *self
    return &tmp
}

func (self *SipReason) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
