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
    "net"
    "strings"

    "sippy/utils"
    "sippy/conf"
)

type SipVia struct {
    compactName
    sipver      string
    host        *sippy_conf.MyAddress
    port        *sippy_conf.MyPort
    extra_headers string

    received    *string
    rport       *string
    ttl         *string
    maddr       *string
    branch      *string
    extension   *string

    received_exists    bool
    rport_exists       bool
    ttl_exists         bool
    maddr_exists       bool
    branch_exists      bool
    extension_exists   bool
}

var _sip_via_name compactName = newCompactName("Via", "v")

func ParseSipVia(body string, config sippy_conf.Config) ([]SipHeader, error) {
    vias := make([]SipHeader, 0)
    for _, via := range strings.Split(body, ",") {
        arr := sippy_utils.FieldsN(via, 2)
        if len(arr) != 2 {
            return nil, errors.New("Bad via: '" + via + "'")
        }
        via := &SipVia{
            compactName : _sip_via_name,
            sipver : arr[0],
        }
        arr = strings.Split(arr[1], ";")
        var val *string
        for _, param := range arr[1:] {
            param = strings.TrimSpace(param)
            sparam := strings.SplitN(param, "=", 2)
            val = nil
            if len(sparam) == 2 {
                val = &sparam[1]
            }
            switch sparam[0] {
            case "received":
                via.received = val
                via.received_exists = true
            case "rport":
                via.rport = val
                via.rport_exists = true
            case "ttl":
                via.ttl = val
                via.ttl_exists = true
            case "maddr":
                via.maddr = val
                via.maddr_exists = true
            case "branch":
                via.branch = val
                via.branch_exists = true
            case "extension":
                via.extension = val
                via.extension_exists = true
            default:
                via.extra_headers += ";" + sparam[0]
                if val != nil {
                    via.extra_headers += "=" + *val
                }
            }
        }
        host, port, err := net.SplitHostPort(arr[0])
        if err != nil {
            via.host = sippy_conf.NewMyAddress(arr[0])
            via.port = nil
        } else {
            via.host = sippy_conf.NewMyAddress(host)
            via.port = sippy_conf.NewMyPort(port)
        }
        vias = append(vias, via)
    }
    return vias, nil
}

func NewSipVia(config sippy_conf.Config) *SipVia {
    self := &SipVia{
        compactName : _sip_via_name,
        rport_exists : true,
        sipver      : "SIP/2.0/UDP",
        host        : config.GetMyAddress(),
        port        : config.GetMyPort(),

        received_exists : false,
        ttl_exists      : false,
        maddr_exists    : false,
        branch_exists   : false,
        extension_exists: false,
    }
    return self
}

func (self *SipVia) Body() string {
    return self.LocalBody(nil)
}

func (self *SipVia) LocalBody(hostport *sippy_conf.HostPort) string {
    return self._local_str(hostport)
}

func (self *SipVia) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipVia) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    if compact {
        return self.CompactName() + ":" + self.LocalBody(hostport)
    }
    return self.Name() + ":" + self.LocalBody(hostport)
}

func (self *SipVia) _local_str(hostport *sippy_conf.HostPort) string {
    s := ""
    if hostport != nil && self.host.IsSystemDefault() {
        s = self.sipver + " " + hostport.Host.String()
    } else {
        s = self.sipver + " " + self.host.String()
    }
    if self.port != nil {
        if hostport != nil && self.port.IsSystemDefault() {
            s += ":" + hostport.Port.String()
        } else {
            s += ":" + self.port.String()
        }
    }
    for _, it := range []struct{ key string; val *string; exists bool } {
                            {"received", self.received, self.received_exists },
                            {"rport", self.rport, self.rport_exists },
                            {"ttl", self.ttl, self.ttl_exists },
                            {"maddr", self.maddr, self.maddr_exists },
                            {"branch", self.branch, self.branch_exists },
                            {"extension", self.extension, self.extension_exists },
                        } {
        if it.exists {
            s += ";" + it.key
            if it.val != nil {
                s += "=" + *it.val
            }
        }
    }
    return s + self.extra_headers
}

func (self *SipVia) GetCopy() *SipVia {
    tmp := *self
    if self.received != nil { tmp_s := *self.received; tmp.received = &tmp_s }
    if self.rport != nil { tmp_s := *self.rport; tmp.rport = &tmp_s }
    if self.ttl != nil { tmp_s := *self.ttl; tmp.ttl = &tmp_s }
    if self.maddr != nil { tmp_s := *self.maddr; tmp.maddr = &tmp_s }
    if self.branch != nil { tmp_s := *self.branch; tmp.branch = &tmp_s }
    if self.extension != nil { tmp_s := *self.extension; tmp.extension = &tmp_s }
    return &tmp
}

func (self *SipVia) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}

func (self *SipVia) GenBranch() {
    buf := make([]byte, 16)
    rand.Read(buf)
    tmp := "z9hG4bK" + fmt.Sprintf("%x", buf)
    self.branch = &tmp
    self.branch_exists = true
}

func (self *SipVia) GetBranch() string {
    if self.branch_exists && self.branch != nil {
        return *self.branch
    }
    return ""
}

func (self *SipVia) GetAddr(config sippy_conf.Config) (string, string) {
    if self.port == nil {
        return self.host.String(), config.SipPort().String()
    } else {
        return self.host.String(), self.port.String()
    }
}

func (self *SipVia) GetTAddr(config sippy_conf.Config) *sippy_conf.HostPort {
    var host, rport string

    if self.rport_exists && self.rport != nil {
        rport = *self.rport
    } else {
        _, rport = self.GetAddr(config)
    }
    if self.received_exists && self.received != nil {
        host = *self.received
    } else {
        host, _ = self.GetAddr(config)
    }
    return sippy_conf.NewHostPort(host, rport)
}

func (self *SipVia) SetRport(v *string) {
    self.rport_exists = true
    self.rport = v
}

func (self *SipVia) SetReceived(v string) {
    self.received_exists = true
    self.received = &v
}

func (self *SipVia) HasRport() bool {
    return self.rport_exists
}
