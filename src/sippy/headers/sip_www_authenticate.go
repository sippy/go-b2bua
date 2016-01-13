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
    "errors"
    "strings"

    "sippy/conf"
    "sippy/utils"
)

type SipWWWAuthenticate struct {
    normalName
    realm *sippy_conf.MyAddress
    nonce string
}

var _sip_www_authenticate_name normalName = newNormalName("WWW-Authenticate")

func ParseSipWWWAuthenticate(body string) ([]SipHeader, error) {
    self, err := NewSipWWWAuthenticateFromString(body)
    if err != nil { return nil, err }
    return []SipHeader{ self }, nil
}

func NewSipWWWAuthenticateFromString(body string) (*SipWWWAuthenticate, error) {
    tmp := sippy_utils.FieldsN(body, 2)
    if len(tmp) != 2 {
        return nil, errors.New("Error parsing authentication (1)")
    }
    self := &SipWWWAuthenticate{
        normalName : _sip_www_authenticate_name,
    }
    for _, part := range strings.Split(tmp[1], ",") {
        arr := strings.SplitN(strings.TrimSpace(part), "=", 2)
        if len(arr) != 2 { continue }
        switch arr[0] {
        case "realm":
            self.realm = sippy_conf.NewMyAddress(strings.Trim(arr[1], "\""))
        case "nonce":
            self.nonce = strings.Trim(arr[1], "\"")
        }
    }
    return self, nil
}

func (self *SipWWWAuthenticate) Body() string {
    return self.LocalBody(nil)
}

func (self *SipWWWAuthenticate) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipWWWAuthenticate) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    return self.Name() + ": " + self.LocalBody(hostport)
}

func (self *SipWWWAuthenticate) LocalBody(hostport *sippy_conf.HostPort) string {
    if hostport != nil && self.realm.IsSystemDefault() {
        return "Digest realm=\"" + hostport.Host.String() + "\",nonce=\"" + self.nonce + "\""
    }
    return "Digest realm=\"" + self.realm.String() + "\",nonce=\"" + self.nonce + "\""
}

func (self *SipWWWAuthenticate) GetRealm() string {
    return self.realm.String()
}

func (self *SipWWWAuthenticate) GetNonce() string {
    return self.nonce
}

func (self *SipWWWAuthenticate) GetCopy() *SipWWWAuthenticate {
    tmp := *self
    return &tmp
}

func (self *SipWWWAuthenticate) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
