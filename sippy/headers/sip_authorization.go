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
    "encoding/hex"
    "errors"
    "strings"

    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/security"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/utils"
)

type SipAuthorizationBody struct {
    username    string
    realm       string
    nonce       string
    uri         string
    response    string
    qop         string
    nc          string
    cnonce      string
    algorithm   string
    opaque      string
    otherparams string
}

type SipAuthorization struct {
    normalName
    body        *SipAuthorizationBody
    string_body string
}

var _sip_authorization_name normalName = newNormalName("Authorization")

func NewSipAuthorizationWithBody(body *SipAuthorizationBody) *SipAuthorization {
    return &SipAuthorization{
        normalName  : _sip_authorization_name,
        body        : body,
    }
}

func NewSipAuthorization(realm, nonce, uri, username, algorithm string) *SipAuthorization {
    return &SipAuthorization{
        normalName : _sip_authorization_name,
        body    : &SipAuthorizationBody{
            realm       : realm,
            nonce       : nonce,
            uri         : uri,
            username    : username,
            algorithm   : algorithm,
        },
    }
}

func CreateSipAuthorization(body string) []SipHeader {
    self := createSipAuthorizationObj(body)
    return []SipHeader{ self }
}

func createSipAuthorizationObj(body string) *SipAuthorization {
    return &SipAuthorization{
        normalName  : _sip_authorization_name,
        string_body : body,
    }
}

func newSipAuthorizationBody(realm, nonce, uri, username, algorithm string) *SipAuthorizationBody {
    return &SipAuthorizationBody{
        realm       : realm,
        nonce       : nonce,
        uri         : uri,
        username    : username,
        algorithm   : algorithm,
    }
}

func parseSipAuthorizationBody(body string) (*SipAuthorizationBody, error) {
    self := &SipAuthorizationBody{
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
        switch strings.ToLower(name) {
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
        case "algorithm":
            self.algorithm = strings.Trim(value, "\"")
        case "opaque":
            self.opaque = strings.Trim(value, "\"")
        default:
            self.otherparams += "," + param
        }
    }
    return self, nil
}

func (self *SipAuthorizationBody) String() string {
    rval := "Digest username=\"" + self.username + "\",realm=\"" + self.realm + "\",nonce=\"" + self.nonce +
        "\",uri=\"" + self.uri + "\",response=\"" + self.response + "\""
    if self.algorithm != "" {
        rval += ",algorithm=\"" + self.algorithm + "\""
    }
    if self.qop != "" {
        rval += ",nc=\"" + self.nc + "\",cnonce=\"" + self.cnonce + "\",qop=" + self.qop
    }
    if self.opaque != "" {
        rval += ",opaque=\"" + self.opaque + "\""
    }
    return rval + self.otherparams
}

func (self *SipAuthorizationBody) GetCopy() *SipAuthorizationBody {
    rval := *self
    return &rval
}

func (self *SipAuthorizationBody) GetNonce() string {
    return self.nonce
}

func (self *SipAuthorizationBody) GetRealm() string {
    return self.realm
}

func (self *SipAuthorizationBody) GetResponse() string {
    return self.response
}

func (self *SipAuthorizationBody) GetUsername() string {
    return self.username
}

func (self *SipAuthorizationBody) GetUri() string {
    return self.uri
}

func (self *SipAuthorizationBody) Verify(passwd, method, entity_body string) bool {
    alg := sippy_security.GetAlgorithm(self.algorithm)
    if alg == nil {
        return false
    }
    HA1 := DigestCalcHA1(alg, self.algorithm, self.username, self.realm, passwd, self.nonce, self.cnonce)
    return self.VerifyHA1(HA1, method, entity_body)
}

func (self *SipAuthorizationBody) VerifyHA1(HA1, method, entity_body string) bool {
    now, _ := sippy_time.NewMonoTime()
    alg := sippy_security.GetAlgorithm(self.algorithm)
    if alg == nil {
        return false
    }
    if self.qop != "" && self.qop != "auth" {
        return false
    }
    if ! sippy_security.HashOracle.ValidateChallenge(self.nonce, alg.Mask, now.Monot()) {
        return false
    }
    response := DigestCalcResponse(alg, HA1, self.nonce, self.nc, self.cnonce, self.qop, method, self.uri, entity_body)
    return response == self.response
}

func (self *SipAuthorization) String() string {
    return self.Name() + ": " + self.StringBody()
}

func (self *SipAuthorization) LocalStr(*sippy_net.HostPort, bool) string {
    return self.String()
}

func (self *SipAuthorization) StringBody() string {
    if self.body != nil {
        return self.body.String()
    }
    return self.string_body
}

func (self *SipAuthorization) GetCopy() *SipAuthorization {
    if self == nil {
        return nil
    }
    var rval SipAuthorization = *self
    if self.body != nil {
        rval.body = self.body.GetCopy()
    }
    return &rval
}

func (self *SipAuthorization) parse() error {
    body, err := parseSipAuthorizationBody(self.string_body)
    if err != nil {
        return err
    }
    self.body = body
    return nil
}

func (self *SipAuthorization) GetBody() (*SipAuthorizationBody, error) {
    if self.body == nil {
        if err := self.parse(); err != nil {
            return nil, err
        }
    }
    return self.body, nil
}

func (self *SipAuthorization) GetUsername() (string, error) {
    body, err := self.GetBody()
    if err != nil {
        return "", err
    }
    return body.GetUsername(), nil
}

func (self *SipAuthorizationBody) GenResponse(password, method, entity_body string) {
    alg := sippy_security.GetAlgorithm(self.algorithm)
    if alg == nil {
        return
    }
    HA1 := DigestCalcHA1(alg, self.algorithm, self.username, self.realm, password,
          self.nonce, self.cnonce)
    self.response = DigestCalcResponse(alg, HA1, self.nonce,
          self.nc, self.cnonce, self.qop, method, self.uri, entity_body)
}

func (self *SipAuthorization) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func DigestCalcHA1(alg *sippy_security.Algorithm, pszAlg, pszUserName, pszRealm, pszPassword, pszNonce, pszCNonce string) string {
    s := pszUserName + ":" + pszRealm + ":" + pszPassword
    hash := alg.NewHash()
    hash.Write([]byte(s))
    HA1 := hash.Sum(nil)
    if strings.HasSuffix(pszAlg, "-sess") {
        s2 := []byte(hex.EncodeToString(HA1))
        s2 = append(s2, []byte(":" + pszNonce + ":" + pszCNonce)...)
        hash = alg.NewHash()
        hash.Write([]byte(s2))
        HA1 = hash.Sum(nil)
    }
    return hex.EncodeToString(HA1)
}

func DigestCalcResponse(alg *sippy_security.Algorithm, HA1, pszNonce, pszNonceCount, pszCNonce, pszQop, pszMethod, pszDigestUri, pszHEntity string) string {
    s := pszMethod + ":" + pszDigestUri
    if pszQop == "auth-int" {
        hash := alg.NewHash()
        hash.Write([]byte(pszHEntity))
        sum := hash.Sum(nil)
        s += ":" + hex.EncodeToString(sum[:])
    }
    hash := alg.NewHash()
    hash.Write([]byte(s))
    sum := hash.Sum(nil)
    HA2 := hex.EncodeToString(sum[:])
    s = HA1 + ":" + pszNonce + ":"
    if pszNonceCount != "" && pszCNonce != "" { // pszQop:
        s += pszNonceCount + ":" + pszCNonce + ":" + pszQop + ":"
    }
    s += HA2
    hash = alg.NewHash()
    hash.Write([]byte(s))
    sum = hash.Sum(nil)
    return hex.EncodeToString(sum[:])
}
