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
    "fmt"
    "os"
    "strings"

    "sippy/conf"
    "sippy/utils"
)

type SipWarning struct {
    normalName
    code        string
    agent       string
    text        string
}

var _sip_warning_name normalName = newNormalName("Warning")

func ParseSipWarning(body string, config sippy_conf.Config) ([]SipHeader, error) {
    self := &SipWarning{
        normalName : _sip_warning_name,
    }
    arr := sippy_utils.FieldsN(body, 3)
    if len(arr) != 3 {
        return nil, fmt.Errorf("Malformed Warning field")
    }
    self.code, self.agent, self.text = arr[0], arr[1], arr[2]
    self.text = strings.Trim(self.text, "\"")
    //self.parsed = true
    return []SipHeader{ self }, nil
}

func NewSipWarning(text string) *SipWarning {
    text = strings.Replace(text, "\"", "'", -1)
    self := &SipWarning{
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

func (self *SipWarning) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    return self.String()
}

func (self *SipWarning) String() string {
    return self.Name() + ": " + self.Body()
}

func (self *SipWarning) Body() string {
    return fmt.Sprintf("%s %s \"%s\"", self.code, self.agent, self.text)
}

func (self *SipWarning) GetCopy() *SipWarning {
    tmp := *self
    return &tmp
}

func (self *SipWarning) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
