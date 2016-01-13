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
    "strings"

    "sippy/conf"
)

type SipReplaces struct {
    normalName
    call_id     string
    from_tag    string
    to_tag      string
    early_only  bool
    otherparams string
}

var _sip_replaces_name normalName = newNormalName("Replaces")

func ParseSipReplaces(body string) ([]SipHeader, error) {
    self := &SipReplaces{
        normalName : _sip_replaces_name,
    }
    params := strings.Split(body, ";")
    self.call_id = params[0]
    for _, param := range params[1:] {
        kv := strings.SplitN(param, "=", 2)
        switch kv[0] {
        case "from-tag":
            if len(kv) == 2 { self.from_tag = kv[1] }
        case "to-tag":
            if len(kv) == 2 { self.to_tag = kv[1] }
        case "early-only":
            self.early_only = true
        default:
            self.otherparams += ";" + param
        }
    }
    return []SipHeader{ self }, nil
}

func (self *SipReplaces) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipReplaces) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    res := self.call_id + ";from-tag=" + self.from_tag + ";to-tag=" + self.to_tag
    if self.early_only {
        res += ";early-only"
    }
    return self.Name() + ": " + res + self.otherparams
}

func (self *SipReplaces) GetCopy() *SipReplaces {
    tmp := *self
    return &tmp
}

func (self *SipReplaces) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
