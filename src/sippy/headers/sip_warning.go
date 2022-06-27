// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2014 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2016 Andriy Pylypenko. All rights reserved.
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
    "os"
    "strings"

    "sippy/net"
    "sippy/utils"
)

type sipWarningBody struct {
    code        string
    agent       string
    text        string
}

type SipWarning struct {
    normalName
    string_body     string
    body            *sipWarningBody
}

var _sip_warning_name normalName = newNormalName("Warning")

func CreateSipWarning(body string) []SipHeader {
    return []SipHeader{
        &SipWarning{
            normalName : _sip_warning_name,
            string_body : body,
        },
    }
}

func (self *SipWarning) parse() error {
    arr := sippy_utils.FieldsN(self.string_body, 3)
    if len(arr) != 3 {
        return errors.New("Malformed Warning field")
    }
    self.body = &sipWarningBody{
        code    : arr[0],
        agent   : arr[1],
        text    : strings.Trim(arr[2], "\""),
    }
    return nil
}

func NewSipWarning(text string) *SipWarning {
    return &SipWarning{
        normalName  : _sip_warning_name,
        body        : newSipWarningBody(text),
    }
}

func newSipWarningBody(text string) *sipWarningBody {
    text = strings.Replace(text, "\"", "'", -1)
    self := &sipWarningBody{
        code    : "399",
        agent   : "unknown",
        text    : text,
    }
    hostname, err := os.Hostname()
    if err == nil {
        self.agent = hostname
    }
    return self
}

func (self *SipWarning) LocalStr(hostport *sippy_net.HostPort, compact bool) string {
    return self.String()
}

func (self *SipWarning) String() string {
    return self.Name() + ": " + self.StringBody()
}

func (self *SipWarning) StringBody() string {
    if self.body != nil {
        return self.body.String()
    }
    return self.string_body
}

func (self *sipWarningBody) String() string {
    return self.code + " " + self.agent + " \"" + self.text + "\""
}

func (self *SipWarning) GetCopy() *SipWarning {
    tmp := *self
    return &tmp
}

func (self *SipWarning) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
