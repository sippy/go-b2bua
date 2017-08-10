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
    "fmt"
    "strconv"
    "strings"

    "sippy/conf"
    "sippy/utils"
)

const (
    RFC3261_USER_UNRESERVED = "&=+$,;?/#"
    RFC3261_UNRESERVED = "-_.!~*'()"
)

var user_enc *sippy_utils.UrlEncode
var passw_enc *sippy_utils.UrlEncode
var hnv_enc *sippy_utils.UrlEncode

func init() {
    user_enc = sippy_utils.NewUrlEncode([]byte(RFC3261_USER_UNRESERVED + RFC3261_UNRESERVED))
    passw_enc = sippy_utils.NewUrlEncode([]byte(RFC3261_UNRESERVED + "&=+$,"))
    //param_enc = sippy_utils.NewUrlEncode([]byte(RFC3261_UNRESERVED + "[]/:&+$"))
    hnv_enc = sippy_utils.NewUrlEncode([]byte(RFC3261_UNRESERVED + "[]/?:+$"))
}

type SipURL struct {
    Username    string
    password    string
    ttl         int
    Host        *sippy_conf.MyAddress
    Port        *sippy_conf.MyPort
    usertype    string
    transport   string
    maddr       string
    method      string
    tag         string
    Lr          bool
    other       []string
    userparams  []string
    headers     map[string]string
    scheme      string
    Q           float64
}

func NewSipURL(username string, host *sippy_conf.MyAddress, port *sippy_conf.MyPort, lr bool /* false */) *SipURL {
    self := &SipURL{
        scheme      : "sip",
        other       : make([]string, 0),
        userparams  : make([]string, 0),
        Username    : username,
        headers     : make(map[string]string),
        Lr          : lr,
        ttl         : -1,
        Host        : host,
        Port        : port,
        Q           : 1,
    }
    return self
}

func ParseSipURL(url string, relaxedparser bool, config sippy_conf.Config) (*SipURL, error) {

    parts := strings.SplitN(url, ":", 2)
    if len(parts) != 2 {
        return nil, errors.New("scheme is not present")
    }
    self := NewSipURL("", nil, nil, false)
    self.scheme = strings.ToLower(parts[0])
    switch self.scheme {
    case "sip": fallthrough
    case "sips":
        return self, self.parseSipURL(parts[1], relaxedparser)
    case "tel":
        if config.AutoConvertTelUrl() {
            self.convertTelUrl(parts[1], relaxedparser, config)
            return self, nil
        }
    }
    return nil, errors.New("unsupported scheme: " + self.scheme + ":")
}

func (self *SipURL) convertTelUrl(url string, relaxedparser bool, config sippy_conf.Config) {
    self.scheme = "sip"

    if relaxedparser {
        self.Host = sippy_conf.NewMyAddress("")
    } else {
        self.Host = config.GetMyAddress()
        self.Port = config.GetMyPort()
    }
    parts := strings.Split(url, ";")
    self.Username, _ = user_enc.Unescape(parts[0])
    if len(parts) > 1 {
        // parse userparams
        for _, part := range parts[1:] {
            // The RFC-3261 suggests the user parameter keys should
            // be converted to lower case.
            arr := strings.SplitN(part, "=", 2)
            if len(arr) == 2 {
                self.userparams = append(self.userparams, strings.ToLower(arr[0]) + "=" + arr[1])
            } else {
                self.userparams = append(self.userparams, part)
            }
        }
    }
}

func (self *SipURL) parseSipURL(url string, relaxedparser bool) error {
    var params, arr []string
    var hostport string

    ear := strings.Index(url, "@") + 1
    parts := strings.Split(url[ear:], ";")
    userdomain := url[0:ear] + parts[0]
    if len(parts) > 1 {
        params = parts[1:]
    } else {
        params = make([]string, 0)
    }
    if len(params) == 0 && strings.Contains(userdomain[ear:], "?") {
        arr = strings.SplitN(userdomain[ear:], "?", 2)
        userdomain_suff := arr[0]
        headers := arr[1]
        userdomain = userdomain[:ear] + userdomain_suff
        for _, header := range strings.Split(headers, "&") {
            arr = strings.SplitN(header, "=", 2)
            if len(arr) == 2 {
                self.headers[strings.ToLower(arr[0])], _ = hnv_enc.Unescape(arr[1])
            }
        }
    }
    if ear > 0 {
        userpass := userdomain[:ear - 1]
        hostport = userdomain[ear:]
        upparts := strings.SplitN(userpass, ":", 2)
        if len(upparts) > 1 {
            self.password, _ = passw_enc.Unescape(upparts[1])
        }
        uparts := strings.Split(upparts[0], ";")
        if len(uparts) > 1 {
            self.userparams = uparts[1:]
        }
        self.Username, _ = user_enc.Unescape(uparts[0])
    } else {
        hostport = userdomain
    }
    var parseport *string = nil
    if relaxedparser && len(hostport) == 0 {
        self.Host = sippy_conf.NewMyAddress("")
    } else if hostport[0] == '[' {
        // IPv6 host
        hpparts := strings.SplitN(hostport, "]", 2)
        self.Host = sippy_conf.NewMyAddress(hpparts[0] + "]")
        if len(hpparts[1]) > 0 {
            hpparts = strings.SplitN(hpparts[1], ":", 2)
            if len(hpparts) > 1 {
                parseport = &hpparts[1]
            }
        }
    } else {
        // IPv4 host
        hpparts := strings.SplitN(hostport, ":", 2)
        if len(hpparts) == 1 {
            self.Host = sippy_conf.NewMyAddress(hpparts[0])
        } else {
            self.Host = sippy_conf.NewMyAddress(hpparts[0])
            parseport = &hpparts[1]
        }
    }
    if parseport != nil {
        port := strings.TrimSpace(*parseport)
        if port == "" {
            // Bug on the other side, work around it
            //print 'WARNING: non-compliant URI detected, empty port number, ' \
            //  'assuming default: "%s"' % str(original_uri)
        } else {
            _, err := strconv.Atoi(port)
            if err != nil {
                if strings.Contains(port, ":") {
                    // Can't parse port number, check why
                    pparts := strings.SplitN(port, ":", 2)
                    if pparts[0] == pparts[1] {
                        // Bug on the other side, work around it
                        //print 'WARNING: non-compliant URI detected, duplicate port number, ' \
                        //  'taking "%s": %s' % (pparts[0], str(original_uri))
                        if _, err = strconv.Atoi(pparts[0]); err != nil {
                            return err
                        }
                        self.Port = sippy_conf.NewMyPort(pparts[0])
                    }
                } else {
                    return err
                }
            } else {
                self.Port = sippy_conf.NewMyPort(port)
            }
        }
    }
    for _, p := range params {
        if p == params[len(params)-1] && strings.Contains(p, "?") {
            self.headers = make(map[string]string)
            arr = strings.SplitN(p, "?", 2)
            p = arr[0]
            headers := arr[1]
            for _, header := range strings.Split(headers, "&") {
                if arr := strings.SplitN(header, "=", 2); len(arr) == 2 {
                    self.headers[strings.ToLower(arr[0])], _ = hnv_enc.Unescape(arr[1])
                }
            }
        }
        nv := strings.SplitN(p, "=", 2)
        if len(nv) == 1 {
            if p == "lr" {
                self.Lr = true
            } else {
                self.other = append(self.other, p)
            }
            continue
        }
        name := nv[0]
        value := nv[1]
        switch name {
        case "user":
            self.usertype = value
        case "transport":
            self.transport = value
        case "ttl":
            if v, err := strconv.Atoi(value); err == nil {
                self.ttl = v
            }
        case "maddr":
            self.maddr = value
        case "method":
            self.method = value
        case "tag":
            self.tag = value
        case "lr":
            // RFC 3261 doesn't allow lr parameter to have a value,
            // but many stupid implementation do it anyway
            self.Lr = true
        case "q":
            if q, err := strconv.ParseFloat(value, 64); err == nil {
                self.Q = q
            }
        default:
            self.other = append(self.other, p)
        }
    }
    return nil
}

func (self *SipURL) String() string {
    return self.LocalStr(nil)
}

func (self *SipURL) LocalStr(hostport *sippy_conf.HostPort) string {
    l := self.scheme + ":"
    if self.Username != "" {
        username := user_enc.Escape(self.Username)
        l += username
        for _, v := range self.userparams {
            l += ";" + v
        }
        if self.password != "" {
            l += ":" + passw_enc.Escape(self.password)
        }
        l += "@"
    }
    if hostport != nil && self.Host.IsSystemDefault() {
        l += hostport.Host.String()
    } else {
        l += self.Host.String()
    }
    if self.Port != nil {
        if hostport != nil && self.Port.IsSystemDefault() {
            l += ":" + hostport.Port.String()
        } else {
            l += ":" + self.Port.String()
        }
    }
    if self.usertype != "" {
        l += ";user=" + self.usertype
    }
    if self.transport != "" { l += ";transport=" + self.transport }
    if self.maddr != ""     { l += ";maddr=" + self.maddr }
    if self.method != ""    { l += ";method=" + self.method }
    if self.tag != ""       { l += ";tag=" + self.tag }
    if self.ttl != -1       { l += fmt.Sprintf(";ttl=%d", self.ttl) }
    for _, v := range self.other {
        l += ";" + v
    }
    if self.Lr {
        l += ";lr"
    }
    if self.Q != 1 {
        l += fmt.Sprintf(";q=%g", self.Q)
    }
    if len(self.headers) > 0 {
        l += "?"
        arr := []string{}
        for k, v := range self.headers {
            arr = append(arr, strings.Title(k) + "=" + hnv_enc.Escape(v))
        }
        l += strings.Join(arr, "&")
    }
    return l
}

func (self *SipURL) GetCopy() *SipURL {
    ret := *self
    return &ret
}

func (self *SipURL) GetAddr(config sippy_conf.Config) *sippy_conf.HostPort {
    if self.Port != nil {
        return sippy_conf.NewHostPort(self.Host.String(), self.Port.String())
    }
    return sippy_conf.NewHostPort(self.Host.String(), config.SipPort().String())
}

func (self *SipURL) SetUserparams(userparams []string) {
    self.userparams = userparams
}

func (self *SipURL) GetUserparams() []string {
    return self.userparams
}
