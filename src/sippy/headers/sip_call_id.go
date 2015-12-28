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
    "crypto/md5"
    "crypto/rand"
    "fmt"
    "time"

    "sippy/conf"
)

type SipCallId struct {
    CallId string
}

func ParseSipCallId(body string) ([]SipHeader, error) {
    self := &SipCallId{
        CallId : body,
    }
    return []SipHeader{ self }, nil
}

func (self *SipCallId) genCallId(config sippy_conf.Config) {
    buf := make([]byte, 20)
    rand.Read(buf)
    buf2, err := time.Now().MarshalBinary()
    if err == nil {
        buf = append(buf, buf2...)
    }
    self.CallId = fmt.Sprintf("%x", md5.Sum(buf)) + "@" + config.GetMyAddress().String()
}

func NewSipCallId(config sippy_conf.Config) *SipCallId {
    self := &SipCallId{}
    self.genCallId(config)
    return self
}

func (self *SipCallId) GetCopy() *SipCallId {
    tmp := *self
    return &tmp
}

func (self *SipCallId) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipCallId) String() string {
    return "Call-Id: " + self.CallId
}

func (self *SipCallId) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    if compact {
        return "i: " + self.CallId
    }
    return self.String()
}
