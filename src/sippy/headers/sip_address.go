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
    "unicode"

    "sippy/conf"
)

type sipAddress struct {
    params      map[string]*string
    url         *SipURL
    hadbrace    bool
    name        string
}

func ParseSipAddress(address string, relaxedparser bool) (*sipAddress, error) {
    var err error
    var arr []string

    // simple 'sip:foo' case
    self := &sipAddress{
        params : make(map[string]*string),
    }

    if strings.HasPrefix(strings.ToLower(address), "sip:") && strings.Index(address, "<") == -1 {
        parts := strings.SplitN(address, ";", 2)
        self.url, err = ParseSipURL(parts[0], relaxedparser)
        if err != nil {
            return nil, err
        }
        if len(parts) == 2 {
            if err = self._parse_paramstring(parts[1]); err != nil {
                return nil, err
            }
        }
        self.hadbrace = false
        return self, nil
    }
    var url *string = nil
    if address[0] == '"' {
        equote := strings.Index(address[1:], "\"") + 1
        if equote != 0 {
            sbrace := strings.Index(address[equote:], "<")
            if sbrace != -1 {
                self.hadbrace = true
                self.name = strings.TrimSpace(address[1:equote])
                tmp := address[equote + sbrace + 1:]
                url = &tmp
            }
        }
    }
    if url == nil {
        arr = strings.SplitN(address, "<", 2)
        if len(arr) != 2 {
            return nil, errors.New("ParseSipAddress #1")
        }
        self.name = strings.TrimSpace(arr[0])
        url = &arr[1]
        if len(self.name) > 0 && self.name[0] == '"' {
            self.name = self.name[1:]
        }
        if len(self.name) > 0 && self.name[len(self.name)-1] == '"' {
            self.name = self.name[:len(self.name)-1]
        }
    }
    arr = strings.SplitN(*url, ">", 2)
    if len(arr) != 2 {
        return nil, errors.New("ParseSipAddress #2")
    }
    paramstring := arr[1]
    self.url, err = ParseSipURL(arr[0], relaxedparser)
    paramstring = strings.TrimSpace(paramstring)
    if err = self._parse_paramstring(paramstring); err != nil {
        return nil, err
    }
    return self, nil
}

func (self *sipAddress) _parse_paramstring(s string) error {
    for _, l := range strings.Split(s, ";") {
        var k string
        var v *string

        if l == "" {
            continue
        }
        arr := strings.SplitN(l, "=", 2)
        if len(arr) == 2 {
            k = strings.ToLower(arr[0])
            tmp := strings.ToLower(arr[1])
            v = &tmp
        } else {
            k = strings.ToLower(arr[0])
            v = nil
        }
        _, ok := self.params[k]
        if ok {
            return errors.New("Duplicate parameter in SIP address: " + k)
        }
        self.params[k] = v
    }
    return nil
}

func (self *sipAddress) String() string {
    return self.LocalStr(nil)
}

func (self *sipAddress) LocalStr(hostport *sippy_conf.HostPort) string {
    var od, cd, s string
    if self.hadbrace {
        od = "<"
        cd = ">"
    }
    if len(self.name) > 0 {
        needs_quote := false
        for _, r := range self.name {
            if unicode.IsLetter(r) || unicode.IsNumber(r) || strings.ContainsRune("-.!%*_+`'~", r) {
                continue
            }
            needs_quote = true
            break
        }
        if needs_quote {
            s += "\"" + self.name + "\" "
        } else {
            s += self.name + " "
        }
        od = "<"
        cd = ">"
    }
    s += od + self.url.LocalStr(hostport) + cd
    for k, v := range self.params {
        if v == nil {
            s += ";" + k
        } else {
            s += ";" + k + "=" + *v
        }
    }
    return s
}

func NewSipAddress(name string, url *SipURL) *sipAddress {
    return &sipAddress{
        name : name,
        url : url,
        hadbrace : true,
        params : make(map[string]*string),
    }
}

func (self *sipAddress) getCopy() *sipAddress {
    ret := *self
    ret.params = make(map[string]*string)
    for k, v := range self.params {
        if v == nil {
            ret.params[k] = nil
        } else {
            s := *v
            ret.params[k] = &s
        }
    }
    ret.url = self.url.GetCopy()
    return &ret
}

func (self *sipAddress) getParam(name string) string {
    ret, ok := self.params[name]
    if !ok || ret == nil {
        return ""
    }
    return *ret
}

func (self *sipAddress) setParam(name, value string) {
    self.params[name] = &value
}

func (self *sipAddress) GetName() string {
    return self.name
}
