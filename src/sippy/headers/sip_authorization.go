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
    "errors"
    "fmt"
    "strings"

    "sippy/conf"
    "sippy/utils"
)

type NewSipXXXAuthorizationFunc func(realm, nonce, method, uri, username, password string) SipHeader

type SipAuthorization struct {
    normalName
    username    string
    realm       string
    nonce       string
    uri         string
    response    string
    qop         string
    nc          string
    cnonce      string
    otherparams string
}

var _sip_authorization_name normalName = newNormalName("Authorization")

func NewSipAuthorization(realm, nonce, method, uri, username, password string) *SipAuthorization {
    HA1 := DigestCalcHA1("md5", username, realm, password, nonce, "")
    response := DigestCalcResponse(HA1, nonce, "", "", "", method, uri, "")
    return &SipAuthorization{
        normalName : _sip_authorization_name,
        realm   : realm,
        nonce   : nonce,
        uri     : uri,
        username : username,
        response : response,
    }
}

func ParseSipAuthorization(body string) ([]SipHeader, error) {
    self, err := NewSipAuthorizationFromString(body)
    if err != nil { return nil, err }
    return []SipHeader{ self }, nil
}

func NewSipAuthorizationFromString(body string) (*SipAuthorization, error) {
    self := &SipAuthorization{
        normalName : _sip_authorization_name,
    }
    arr := sippy_utils.FieldsN(body, 2)
    if len(arr) != 2 {
        return nil, errors.New("Error parsing authorization (1)")
    }
    for _, param := range strings.Split(arr[1], ",") {
        kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
        if len(kv) != 2 {
            return nil, errors.New("Error parsing authorization (2)")
        }
        name, value := kv[0], kv[1]
        switch name {
        case "username":
            self.username = strings.Trim(value, "\"")
        case "uri":
            self.uri = strings.Trim(value, "\"")
        case "realm":
            self.realm = strings.Trim(value, "\"")
        case "nonce":
            self.nonce = strings.Trim(value, "\"")
        case "response":
            self.response = strings.Trim(value, "\"")
        case "qop":
            self.qop = strings.Trim(value, "\"")
        case "cnonce":
            self.cnonce = strings.Trim(value, "\"")
        case "nc":
            self.nc = strings.Trim(value, "\"")
        default:
            self.otherparams += "," + param
        }
    }
    return self, nil
}

func (self *SipAuthorization) String() string {
    return self.Name() + ": " + self.Body()
}

func (self *SipAuthorization) LocalStr(*sippy_conf.HostPort, bool) string {
    return self.String()
}

func (self *SipAuthorization) Body() string {
    rval := "Digest username=\"" + self.username + "\",realm=\"" + self.realm + "\",nonce=\"" + self.nonce +
        "\",uri=\"" + self.uri + "\",response=\"" + self.response + "\""
    if self.qop != "" {
        rval += ",nc=\"" + self.nc + "\",cnonce=\"" + self.cnonce + "\",qop=" + self.qop
    }
    return rval + self.otherparams
}

func (self *SipAuthorization) GetCopy() *SipAuthorization {
    if self == nil {
        return nil
    }
    var rval SipAuthorization = *self
    return &rval
}

func (self *SipAuthorization) GetUsername() string {
    return self.username
}

func (self *SipAuthorization) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func DigestCalcHA1(pszAlg, pszUserName, pszRealm, pszPassword, pszNonce, pszCNonce string) string {
    s := pszUserName + ":" + pszRealm + ":" + pszPassword
    HA1 := md5.Sum([]byte(s))
    if pszAlg == "md5-sess" {
        s2 := make([]byte, len(HA1))
        for i, b := range HA1 {
            s2[i] = b
        }
        s2 = append(s2, []byte(":" + pszNonce + ":" + pszCNonce)...)
        HA1 = md5.Sum(s2)
    }
    return fmt.Sprintf("%x", HA1)
}

func DigestCalcResponse(HA1, pszNonce string, pszNonceCount, pszCNonce, pszQop, pszMethod, pszDigestUri, pszHEntity string) string {
    s := pszMethod + ":" + pszDigestUri
    if pszQop == "auth-int" {
        s += ":" + pszHEntity
    }
    HA2 := fmt.Sprintf("%x", md5.Sum([]byte(s)))
    s = HA1 + ":" + pszNonce + ":"
    if pszNonceCount != "" && pszCNonce != "" { // pszQop:
        s += pszNonceCount + ":" + pszCNonce + ":" + pszQop + ":"
    }
    s += HA2
    return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func (self *SipAuthorization) VerifyHA1(HA1, method string) bool {
    response := DigestCalcResponse(HA1, self.nonce, self.nc, self.cnonce, self.qop, method, self.uri, "")
    return response == self.response
}
