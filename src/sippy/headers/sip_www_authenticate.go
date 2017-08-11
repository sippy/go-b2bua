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
    "crypto/rand"
    "errors"
    "fmt"
    "strings"

    "sippy/conf"
    "sippy/utils"
)

type sipWWWAuthenticateBody struct {
    realm *sippy_conf.MyAddress
    nonce string
}

type SipWWWAuthenticate struct {
    normalName
    string_body     string
    body            *sipWWWAuthenticateBody
}

var _sip_www_authenticate_name normalName = newNormalName("WWW-Authenticate")

func CreateSipWWWAuthenticate(body string, config sippy_conf.Config) []SipHeader {
    return []SipHeader{ createSipWWWAuthenticateObj(body) }
}

func NewSipWWWAuthenticateWithRealm(realm string) *SipWWWAuthenticate {
    return &SipWWWAuthenticate{
        normalName  : _sip_www_authenticate_name,
        body        : newSipWWWAutenticateBody(realm),
    }
}

func newSipWWWAutenticateBody(realm string) *sipWWWAuthenticateBody {
    buf := make([]byte, 20)
    rand.Read(buf)
    return &sipWWWAuthenticateBody{
        realm : sippy_conf.NewMyAddress(realm),
        nonce : fmt.Sprintf("%x", buf),
    }
}

func createSipWWWAuthenticateObj(body string) *SipWWWAuthenticate {
    return &SipWWWAuthenticate{
        string_body     : body,
    }
}

func (self *SipWWWAuthenticate) parse() error {
    tmp := sippy_utils.FieldsN(self.string_body, 2)
    if len(tmp) != 2 {
        return errors.New("Error parsing authentication (1)")
    }
    body := &sipWWWAuthenticateBody{}
    for _, part := range strings.Split(tmp[1], ",") {
        arr := strings.SplitN(strings.TrimSpace(part), "=", 2)
        if len(arr) != 2 { continue }
        switch arr[0] {
        case "realm":
            body.realm = sippy_conf.NewMyAddress(strings.Trim(arr[1], "\""))
        case "nonce":
            body.nonce = strings.Trim(arr[1], "\"")
        }
    }
    self.body = body
    return nil
}

func (self *SipWWWAuthenticate) StringBody() string {
    return self.LocalStringBody(nil)
}

func (self *SipWWWAuthenticate) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipWWWAuthenticate) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    return self.Name() + ": " + self.LocalStringBody(hostport)
}

func (self *SipWWWAuthenticate) LocalStringBody(hostport *sippy_conf.HostPort) string {
    if self.body != nil {
        return self.body.localString(hostport)
    }
    return self.string_body
}

func (self *sipWWWAuthenticateBody) localString(hostport *sippy_conf.HostPort) string {
    if hostport != nil && self.realm.IsSystemDefault() {
        return "Digest realm=\"" + hostport.Host.String() + "\",nonce=\"" + self.nonce + "\""
    }
    return "Digest realm=\"" + self.realm.String() + "\",nonce=\"" + self.nonce + "\""
}
/*
func (self *SipWWWAuthenticate) GetRealm() (string, error) {
    if self.body == nil {
        if err := self.parse(); err != nil {
            return "", err
        }
    }
    return self.body.realm.String(), nil
}

func (self *SipWWWAuthenticate) GetNonce() (string, error) {
    if self.body == nil {
        if err := self.parse(); err != nil {
            return "", err
        }
    }
    return self.body.nonce, nil
}
*/
func (self *SipWWWAuthenticate) GetCopy() *SipWWWAuthenticate {
    tmp := *self
    if self.body != nil {
        self.body = self.body.getCopy()
    }
    return &tmp
}

func (self *sipWWWAuthenticateBody) getCopy() *sipWWWAuthenticateBody {
    tmp := *self
    return &tmp
}

func (self *SipWWWAuthenticate) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
