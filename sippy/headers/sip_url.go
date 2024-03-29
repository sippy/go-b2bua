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
    "strconv"
    "strings"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/utils"
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
    Password    string
    Ttl         int
    Host        *sippy_net.MyAddress
    Port        *sippy_net.MyPort
    Usertype    string
    Transport   string
    Maddr       string
    Method      string
    Tag         string
    Lr          bool
    Other       []string
    Userparams  []string
    Headers     map[string]string
    Scheme      string
}

func NewSipURL(username string, host *sippy_net.MyAddress, port *sippy_net.MyPort, lr bool /* false */) *SipURL {
    self := &SipURL{
        Scheme      : "sip",
        Other       : make([]string, 0),
        Userparams  : make([]string, 0),
        Username    : username,
        Headers     : make(map[string]string),
        Lr          : lr,
        Ttl         : -1,
        Host        : host,
        Port        : port,
    }
    return self
}

func ParseURL(url string, relaxedparser bool) (*SipURL, error) {
    parts := strings.SplitN(url, ":", 2)
    if len(parts) != 2 {
        return nil, errors.New("scheme is not present")
    }
    self := NewSipURL("", nil, nil, false)
    self.Scheme = strings.ToLower(parts[0])
    switch self.Scheme {
    case "sip": fallthrough
    case "sips":
        return self, self.parseSipURL(parts[1], relaxedparser)
    case "tel":
        self.parseTelUrl(parts[1])
        return self, nil
    }
    return nil, errors.New("unsupported scheme: " + self.Scheme + ":")
}

func ParseSipURL(url string, relaxedparser bool, config sippy_conf.Config) (*SipURL, error) {
    self, err := ParseURL(url, relaxedparser)
    if err != nil {
        return nil, err
    }
    if self.Scheme == "tel" {
        if config != nil && config.AutoConvertTelUrl() {
            self.convertTelUrl(relaxedparser, config)
        } else {
            return nil, errors.New("unsupported scheme: " + self.Scheme + ":")
        }
    }
    return self, nil
}

func (self *SipURL) convertTelUrl(relaxedparser bool, config sippy_conf.Config) {
    self.Scheme = "sip"
    if relaxedparser {
        self.Host = sippy_net.NewMyAddress("")
    } else {
        self.Host = config.GetMyAddress()
        self.Port = config.DefaultPort()
    }
}

func (self *SipURL) parseTelUrl(url string) {
    parts := strings.Split(url, ";")
    self.Username, _ = user_enc.Unescape(parts[0])
    if len(parts) > 1 {
        // parse userparams
        for _, part := range parts[1:] {
            // The RFC-3261 suggests the user parameter keys should
            // be converted to lower case.
            arr := strings.SplitN(part, "=", 2)
            if len(arr) == 2 {
                self.Userparams = append(self.Userparams, strings.ToLower(arr[0]) + "=" + arr[1])
            } else {
                self.Userparams = append(self.Userparams, part)
            }
        }
    }
}

func (self *SipURL) parseSipURL(url string, relaxedparser bool) error {
    var params []string
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
        arr := strings.SplitN(userdomain[ear:], "?", 2)
        userdomain_suff := arr[0]
        headers := arr[1]
        userdomain = userdomain[:ear] + userdomain_suff
        for _, header := range strings.Split(headers, "&") {
            arr = strings.SplitN(header, "=", 2)
            if len(arr) == 2 {
                self.Headers[strings.ToLower(arr[0])], _ = hnv_enc.Unescape(arr[1])
            }
        }
    }
    if ear > 0 {
        userpass := userdomain[:ear - 1]
        hostport = userdomain[ear:]
        upparts := strings.SplitN(userpass, ":", 2)
        if len(upparts) > 1 {
            self.Password, _ = passw_enc.Unescape(upparts[1])
        }
        uparts := strings.Split(upparts[0], ";")
        if len(uparts) > 1 {
            self.Userparams = uparts[1:]
        }
        self.Username, _ = user_enc.Unescape(uparts[0])
    } else {
        hostport = userdomain
    }
    var parseport *string = nil
    if relaxedparser && len(hostport) == 0 {
        self.Host = sippy_net.NewMyAddress("")
    } else if hostport[0] == '[' {
        // IPv6 host
        hpparts := strings.SplitN(hostport, "]", 2)
        self.Host = sippy_net.NewMyAddress(hpparts[0] + "]")
        if len(hpparts[1]) > 0 {
            hpparts = strings.SplitN(hpparts[1], ":", 2)
            if len(hpparts) > 1 {
                parseport = &hpparts[1]
            }
        }
    } else {
        // IPv4 host
        hpparts := strings.SplitN(hostport, ":", 2)
        self.Host = sippy_net.NewMyAddress(hpparts[0])
        if len(hpparts) == 2 {
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
                        self.Port = sippy_net.NewMyPort(pparts[0])
                    } else {
                        return err
                    }
                } else {
                    return err
                }
            } else {
                self.Port = sippy_net.NewMyPort(port)
            }
        }
    }
    if len(params) > 0 {
        last_param := params[len(params) - 1]
        arr := strings.SplitN(last_param, "?", 2)
        params[len(params) - 1] = arr[0]
        self.SetParams(params)
        if len(arr) == 2 {
            self.Headers = make(map[string]string)
            headers := arr[1]
            for _, header := range strings.Split(headers, "&") {
                if arr := strings.SplitN(header, "=", 2); len(arr) == 2 {
                    self.Headers[strings.ToLower(arr[0])], _ = hnv_enc.Unescape(arr[1])
                }
            }
        }
    }
    return nil
}

func (self *SipURL) SetParams(params []string) {
    self.Usertype = ""
    self.Transport = ""
    self.Maddr = ""
    self.Method = ""
    self.Tag = ""
    self.Ttl = -1
    self.Other = []string{}
    self.Lr = false

    for _, p := range params {
        nv := strings.SplitN(p, "=", 2)
        if len(nv) == 1 {
            if p == "lr" {
                self.Lr = true
            } else {
                self.Other = append(self.Other, p)
            }
            continue
        }
        name := nv[0]
        value := nv[1]
        switch name {
        case "user":
            self.Usertype = value
        case "transport":
            self.Transport = value
        case "ttl":
            if v, err := strconv.Atoi(value); err == nil {
                self.Ttl = v
            }
        case "maddr":
            self.Maddr = value
        case "method":
            self.Method = value
        case "tag":
            self.Tag = value
        case "lr":
            // RFC 3261 doesn't allow lr parameter to have a value,
            // but many stupid implementation do it anyway
            self.Lr = true
        default:
            self.Other = append(self.Other, p)
        }
    }
}

func (self *SipURL) String() string {
    return self.LocalStr(nil)
}

func (self *SipURL) LocalStr(hostport *sippy_net.HostPort) string {
    l := self.Scheme + ":"
    if self.Username != "" {
        username := user_enc.Escape(self.Username)
        l += username
        for _, v := range self.Userparams {
            l += ";" + v
        }
        if self.Password != "" {
            l += ":" + passw_enc.Escape(self.Password)
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
    for _, p := range self.GetParams() {
        l += ";" + p
    }
    if len(self.Headers) > 0 {
        l += "?"
        arr := []string{}
        for k, v := range self.Headers {
            arr = append(arr, strings.Title(k) + "=" + hnv_enc.Escape(v))
        }
        l += strings.Join(arr, "&")
    }
    return l
}

func (self *SipURL) GetParams() []string {
    ret := []string{}
    if self.Usertype != ""  { ret = append(ret, "user=" + self.Usertype) }
    if self.Transport != "" { ret = append(ret, "transport=" + self.Transport) }
    if self.Maddr != ""     { ret = append(ret, "maddr=" + self.Maddr) }
    if self.Method != ""    { ret = append(ret, "method=" + self.Method) }
    if self.Tag != ""       { ret = append(ret, "tag=" + self.Tag) }
    if self.Ttl != -1       { ret = append(ret, "ttl=" + strconv.Itoa(self.Ttl)) }
    ret = append(ret, self.Other...)
    if self.Lr              { ret = append(ret, "lr") }
    return ret
}

func (self *SipURL) GetCopy() *SipURL {
    ret := *self
    return &ret
}

func (self *SipURL) GetAddr(config sippy_conf.Config) *sippy_net.HostPort {
    if self.Port != nil {
        return sippy_net.NewHostPort(self.Host.String(), self.Port.String())
    }
    return sippy_net.NewHostPort(self.Host.String(), config.DefaultPort().String())
}
