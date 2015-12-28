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
package sippy

import (
    "strings"

    "sippy/conf"
    "sippy/types"
)

type SdpHeader interface {
    String() string
    LocalStr(hostport *sippy_conf.HostPort) string
}

type sdp_header_and_name struct {
    name    string
    header  SdpHeader
}

type sdpBody struct {
    sections        []*sdpMediaDescription
    v_header        *sdpGeneric
    o_header        *sdpOrigin
    s_header        *sdpGeneric
    i_header        *sdpGeneric
    u_header        *sdpGeneric
    e_header        *sdpGeneric
    p_header        *sdpGeneric
    b_header        *sdpGeneric
    t_header        *sdpGeneric
    r_header        *sdpGeneric
    z_header        *sdpGeneric
    k_header        *sdpGeneric
    a_headers       []string
    c_header        *sdpConnecton
}

func ParseSdpBody(body string) *sdpBody {
    self := &sdpBody{
        a_headers       : make([]string, 0),
        sections        : make([]*sdpMediaDescription, 0),
    }
    if body == "" {
        return self
    }
    current_snum := 0
    var c_header *sdpConnecton
    for _, line := range strings.FieldsFunc(strings.TrimSpace(body), func(c rune) bool { return c == '\n' || c == '\r' }) {
        line = strings.TrimSpace(line)
        if line == "" { continue }
        arr := strings.SplitN(line, "=", 2)
        if len(arr) != 2 { continue }
        name, v := strings.ToLower(arr[0]), arr[1]
        if name == "m" {
            current_snum += 1
            self.sections = append(self.sections, NewSdpMediaDescription())
        }
        if current_snum == 0 {
            if name == "c" {
                c_header = ParseSdpConnecton(v)
            } else if name == "a" {
                self.a_headers = append(self.a_headers, v)
            } else {
                switch name {
                case "v":
                    self.v_header = ParseSdpGeneric(v)
                case "o":
                    self.o_header = ParseSdpOrigin(v)
                case "s":
                    self.s_header = ParseSdpGeneric(v)
                case "i":
                    self.i_header = ParseSdpGeneric(v)
                case "u":
                    self.u_header = ParseSdpGeneric(v)
                case "e":
                    self.e_header = ParseSdpGeneric(v)
                case "p":
                    self.p_header = ParseSdpGeneric(v)
                case "b":
                    self.b_header = ParseSdpGeneric(v)
                case "t":
                    self.t_header = ParseSdpGeneric(v)
                case "r":
                    self.r_header = ParseSdpGeneric(v)
                case "z":
                    self.z_header = ParseSdpGeneric(v)
                case "k":
                    self.k_header = ParseSdpGeneric(v)
                }
            }
        } else {
            self.sections[len(self.sections)-1].addHeader(name, v)
        }
    }
    if c_header != nil {
        for _, section := range self.sections {
            if section.c_header == nil {
                section.c_header = c_header
            }
        }
        if len(self.sections) == 0 {
            self.c_header = c_header
        }
    }
    return self
}

func (self *sdpBody) first_half() []*sdp_header_and_name {
    ret := []*sdp_header_and_name{}
    if self.v_header != nil { ret = append(ret, &sdp_header_and_name{ "v", self.v_header }) }
    if self.o_header != nil { ret = append(ret, &sdp_header_and_name{ "o", self.o_header }) }
    if self.s_header != nil { ret = append(ret, &sdp_header_and_name{ "s", self.s_header }) }
    if self.i_header != nil { ret = append(ret, &sdp_header_and_name{ "i", self.i_header }) }
    if self.u_header != nil { ret = append(ret, &sdp_header_and_name{ "u", self.u_header }) }
    if self.e_header != nil { ret = append(ret, &sdp_header_and_name{ "e", self.e_header }) }
    if self.p_header != nil { ret = append(ret, &sdp_header_and_name{ "p", self.p_header }) }
    return ret
}

func (self *sdpBody) second_half() []*sdp_header_and_name {
    ret := []*sdp_header_and_name{}
    if self.b_header != nil { ret = append(ret, &sdp_header_and_name{ "b", self.b_header }) }
    if self.t_header != nil { ret = append(ret, &sdp_header_and_name{ "t", self.t_header }) }
    if self.r_header != nil { ret = append(ret, &sdp_header_and_name{ "r", self.r_header }) }
    if self.z_header != nil { ret = append(ret, &sdp_header_and_name{ "z", self.z_header }) }
    if self.k_header != nil { ret = append(ret, &sdp_header_and_name{ "k", self.k_header }) }
    return ret
}

func (self *sdpBody) all_headers() []*sdp_header_and_name {
    ret := self.first_half()
    if self.c_header != nil { ret = append(ret, &sdp_header_and_name{ "c", self.c_header }) }
    return append(ret, self.second_half()...)
}

func (self *sdpBody) String() string {
    s := ""
    if len(self.sections) == 1 && self.sections[0].c_header != nil {
        for _, it := range self.first_half() {
            s += it.name + "=" + it.header.String() + "\r\n"
        }
        s += "c=" + self.sections[0].c_header.String() + "\r\n"
        for _, it := range self.second_half() {
            s += it.name + "=" + it.header.String() + "\r\n"
        }
        for _, header := range self.a_headers {
            s += "a=" + header + "\r\n"
        }
        s += self.sections[0].LocalStr(nil, true /* noC */)
        return s
    }
    // Special code to optimize for the cases when there are many media streams pointing to
    // the same IP. Only include c= header into the top section of the SDP and remove it from
    // the streams that match.
    optimize_c_headers := false
    sections_0_str := ""
    if len(self.sections) > 1 && self.c_header == nil && self.sections[0].c_header != nil &&
      *self.sections[0].c_header == *self.sections[1].c_header {
        // Special code to optimize for the cases when there are many media streams pointing to
        // the same IP. Only include c= header into the top section of the SDP and remove it from
        // the streams that match.
        optimize_c_headers = true
        sections_0_str = self.sections[0].c_header.String()
    }
    if optimize_c_headers {
        for _, it := range self.first_half() {
            s += it.name + "=" + it.header.String() + "\r\n"
        }
        s += "c=" + sections_0_str + "\r\n"
        for _, it := range self.second_half() {
            s += it.name + "=" + it.header.String() + "\r\n"
        }
    } else {
        for _, it := range self.all_headers() {
            s += it.name + "=" + it.header.String() + "\r\n"
        }
    }
    for _, header := range self.a_headers {
        s += "a=" + header + "\r\n"
    }
    for _, section := range self.sections {
        if optimize_c_headers && section.c_header != nil && section.c_header.String() == sections_0_str {
            s += section.LocalStr(nil, true /* noC */)
        } else {
            s += section.String()
        }
    }
    return s
}

func (self *sdpBody) LocalStr(hostport *sippy_conf.HostPort) string {
    s := ""
    if len(self.sections) == 1 && self.sections[0].c_header != nil {
        for _, it := range self.first_half() {
            s += it.name + "=" + it.header.LocalStr(hostport) + "\r\n"
        }
        s += "c=" + self.sections[0].c_header.LocalStr(hostport) + "\r\n"
        for _, it := range self.second_half() {
            s += it.name + "=" + it.header.LocalStr(hostport) + "\r\n"
        }
        for _, header := range self.a_headers {
            s += "a=" + header + "\r\n"
        }
        s += self.sections[0].LocalStr(hostport, true /* noC */)
        return s
    }
    // Special code to optimize for the cases when there are many media streams pointing to
    // the same IP. Only include c= header into the top section of the SDP and remove it from
    // the streams that match.
    optimize_c_headers := false
    sections_0_str := ""
    if len(self.sections) > 1 && self.c_header == nil && self.sections[0].c_header != nil &&
      self.sections[0].c_header.LocalStr(hostport) == self.sections[1].c_header.LocalStr(hostport) {
        // Special code to optimize for the cases when there are many media streams pointing to
        // the same IP. Only include c= header into the top section of the SDP and remove it from
        // the streams that match.
        optimize_c_headers = true
        sections_0_str = self.sections[0].c_header.LocalStr(hostport)
    }
    if optimize_c_headers {
        for _, it := range self.first_half() {
            s += it.name + "=" + it.header.LocalStr(hostport) + "\r\n"
        }
        s += "c=" + sections_0_str + "\r\n"
        for _, it := range self.second_half() {
            s += it.name + "=" + it.header.LocalStr(hostport) + "\r\n"
        }
    } else {
        for _, it := range self.all_headers() {
            s += it.name + "=" + it.header.LocalStr(hostport) + "\r\n"
        }
    }
    for _, header := range self.a_headers {
        s += "a=" + header + "\r\n"
    }
    for _, section := range self.sections {
        if optimize_c_headers && section.c_header != nil &&
          section.c_header.LocalStr(hostport) == sections_0_str {
            s += section.LocalStr(hostport, /*noC =*/ true)
        } else {
            s += section.LocalStr(hostport, /*noC =*/ false)
        }
    }
    return s
}

func (self *sdpBody) GetCopy() sippy_types.ParsedMsgBody {
    sections := make([]*sdpMediaDescription, len(self.sections))
    for i, s := range self.sections {
        sections[i] = s.GetCopy()
    }
    a_headers := make([]string, len(self.a_headers))
    copy(a_headers, self.a_headers)
    return &sdpBody{
        sections    : sections,
        v_header    : self.v_header.GetCopy(),
        o_header    : self.o_header.GetCopy(),
        s_header    : self.s_header.GetCopy(),
        i_header    : self.i_header.GetCopy(),
        u_header    : self.u_header.GetCopy(),
        e_header    : self.e_header.GetCopy(),
        p_header    : self.p_header.GetCopy(),
        b_header    : self.b_header.GetCopy(),
        t_header    : self.t_header.GetCopy(),
        r_header    : self.r_header.GetCopy(),
        z_header    : self.z_header.GetCopy(),
        k_header    : self.k_header.GetCopy(),
        a_headers   : a_headers,
        c_header    : self.c_header.GetCopy(),
    }
}

func (self *sdpBody) SetCHeaderAddr(addr string) {
    for _, sect := range self.sections {
        sect.SetCHeaderAddr(addr)
    }
}
