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
    "time"

    "sippy/net"
    "sippy/security"
    "sippy/utils"
)

type SipWWWAuthenticateBody struct {
    realm       *sippy_net.MyAddress
    nonce       string
    algorithm   string
    qop         []string
    otherparams []string
    opaque      string
}

type SipWWWAuthenticate struct {
    normalName
    string_body     string
    body            *SipWWWAuthenticateBody
    aclass          func(*SipAuthorizationBody) SipHeader
}

var _sip_www_authenticate_name normalName = newNormalName("WWW-Authenticate")

func CreateSipWWWAuthenticate(body string) []SipHeader {
    return []SipHeader{ createSipWWWAuthenticateObj(body) }
}

func NewSipWWWAuthenticateWithRealm(realm, algorithm string, now_mono time.Time) *SipWWWAuthenticate {
    return &SipWWWAuthenticate{
        normalName  : _sip_www_authenticate_name,
        body        : newSipWWWAutenticateBody(realm, algorithm, now_mono),
        aclass      : func(body *SipAuthorizationBody) SipHeader { return NewSipAuthorizationWithBody(body) },
    }
}

func newSipWWWAutenticateBody(realm, algorithm string, now_mono time.Time) *SipWWWAuthenticateBody {
    self := &SipWWWAuthenticateBody{
        algorithm   : algorithm,
        realm       : sippy_net.NewMyAddress(realm),
    }
    alg := sippy_security.GetAlgorithm(algorithm)
    if alg == nil {
        buf := make([]byte, 20)
        rand.Read(buf)
        self.nonce = fmt.Sprintf("%x", buf)
    } else {
        self.nonce = sippy_security.HashOracle.EmitChallenge(alg.Mask, now_mono)
    }
    return self
}

func createSipWWWAuthenticateObj(body string) *SipWWWAuthenticate {
    return &SipWWWAuthenticate{
        normalName      : _sip_www_authenticate_name,
        string_body     : body,
        aclass          : func(body *SipAuthorizationBody) SipHeader { return NewSipAuthorizationWithBody(body) },
    }
}

func (self *SipWWWAuthenticate) parse() error {
    tmp := sippy_utils.FieldsN(self.string_body, 2)
    if len(tmp) != 2 {
        return errors.New("Error parsing authentication (1)")
    }
    body := &SipWWWAuthenticateBody{}
    for _, part := range strings.Split(tmp[1], ",") {
        arr := strings.SplitN(strings.TrimSpace(part), "=", 2)
        if len(arr) != 2 { continue }
        switch arr[0] {
        case "realm":
            body.realm = sippy_net.NewMyAddress(strings.Trim(arr[1], "\""))
        case "nonce":
            body.nonce = strings.Trim(arr[1], "\"")
        case "opaque":
            body.opaque = strings.Trim(arr[1], "\"")
        case "algorithm":
            body.algorithm = strings.Trim(arr[1], "\"")
        case "qop":
            qops := strings.Trim(arr[1], "\"")
            body.qop = strings.Split(qops, ",")
        default:
            body.otherparams = append(body.otherparams, part)
        }
    }
    self.body = body
    return nil
}

func (self SipWWWAuthenticate) GetBody() (*SipWWWAuthenticateBody, error) {
    if self.body == nil {
        if err := self.parse(); err != nil {
            return nil, err
        }
    }
    return self.body, nil
}

func (self *SipWWWAuthenticate) StringBody() string {
    return self.LocalStringBody(nil)
}

func (self *SipWWWAuthenticate) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipWWWAuthenticate) LocalStr(hostport *sippy_net.HostPort, compact bool) string {
    return self.Name() + ": " + self.LocalStringBody(hostport)
}

func (self *SipWWWAuthenticate) LocalStringBody(hostport *sippy_net.HostPort) string {
    if self.body != nil {
        return self.body.localString(hostport)
    }
    return self.string_body
}

func (self *SipWWWAuthenticateBody) localString(hostport *sippy_net.HostPort) string {
    realm := self.realm.String()
    if hostport != nil && self.realm.IsSystemDefault() {
        realm = hostport.Host.String()
    }
    ret := "Digest realm=\"" + realm + "\",nonce=\"" + self.nonce + "\""
    if self.algorithm != "" {
        ret += ",algorithm=\"" + self.algorithm +"\""
    }
    if self.opaque != "" {
        ret += ",opaque=\"" + self.opaque + "\""
    }
    if len(self.qop) == 1 {
        ret += ",qop=" + self.qop[0]
    } else if len(self.qop) > 1 {
        ret += ",qop=\"" + strings.Join(self.qop, ",") + "\""
    }
    if len(self.otherparams) > 0 {
        ret += "," + strings.Join(self.otherparams, ",")
    }
    return ret
}

func (self *SipWWWAuthenticateBody) GetRealm() string {
    return self.realm.String()
}

func (self *SipWWWAuthenticateBody) GetNonce() string {
    return self.nonce
}

func (self *SipWWWAuthenticate) GetCopy() *SipWWWAuthenticate {
    tmp := *self
    if self.body != nil {
        self.body = self.body.getCopy()
    }
    return &tmp
}

func (self *SipWWWAuthenticateBody) getCopy() *SipWWWAuthenticateBody {
    tmp := *self
    return &tmp
}

func (self *SipWWWAuthenticate) GenAuthHF(username, password, method, uri string) (SipHeader, error) {
    body, err := self.GetBody()
    if err != nil {
        return nil, err
    }
    auth := newSipAuthorizationBody(body.realm.String(), body.nonce, uri, username, body.algorithm)
    if body.qop != nil {
        auth.qop = "auth"
        auth.nc = "00000001"
        buf := make([]byte, 4)
        rand.Read(buf)
        auth.cnonce = fmt.Sprintf("%x", buf)
    }
    if body.opaque != "" {
        auth.opaque = body.opaque
    }
    auth.GenResponse(password, method)
    return self.aclass(auth), nil
}

func (self *SipWWWAuthenticate) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipWWWAuthenticate) Algorithm() (string, error) {
    body, err := self.GetBody()
    if err != nil {
        return "", err
    }
    return body.algorithm, nil
}

func (self *SipWWWAuthenticate) SupportedAlgorithm() (bool, error) {
    body, err := self.GetBody()
    if err != nil {
        return false, err
    }
    alg := sippy_security.GetAlgorithm(body.algorithm)
    return alg != nil, nil
}
