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
    "fmt"
    "strings"

    "sippy/conf"
    "sippy/sdp"
    "sippy/types"
)

type sdpBody struct {
    sections        []*sippy_sdp.SdpMediaDescription
    v_header        *sippy_sdp.SdpGeneric
    o_header        *sippy_sdp.SdpOrigin
    s_header        *sippy_sdp.SdpGeneric
    i_header        *sippy_sdp.SdpGeneric
    u_header        *sippy_sdp.SdpGeneric
    e_header        *sippy_sdp.SdpGeneric
    p_header        *sippy_sdp.SdpGeneric
    b_header        *sippy_sdp.SdpGeneric
    t_header        *sippy_sdp.SdpGeneric
    r_header        *sippy_sdp.SdpGeneric
    z_header        *sippy_sdp.SdpGeneric
    k_header        *sippy_sdp.SdpGeneric
    a_headers       []string
    c_header        *sippy_sdp.SdpConnecton
}

func ParseSdpBody(body string) (*sdpBody, error) {
    var err error
    self := &sdpBody{
        a_headers       : make([]string, 0),
        sections        : make([]*sippy_sdp.SdpMediaDescription, 0),
    }
    current_snum := 0
    var c_header *sippy_sdp.SdpConnecton
    for _, line := range strings.FieldsFunc(strings.TrimSpace(body), func(c rune) bool { return c == '\n' || c == '\r' }) {
        line = strings.TrimSpace(line)
        if line == "" { continue }
        arr := strings.SplitN(line, "=", 2)
        if len(arr) != 2 { continue }
        name, v := strings.ToLower(arr[0]), arr[1]
        if name == "m" {
            current_snum += 1
            self.sections = append(self.sections, sippy_sdp.NewSdpMediaDescription())
        }
        if current_snum == 0 {
            if name == "c" {
                c_header = sippy_sdp.ParseSdpConnecton(v)
            } else if name == "a" {
                self.a_headers = append(self.a_headers, v)
            } else {
                switch name {
                case "v":
                    self.v_header = sippy_sdp.ParseSdpGeneric(v)
                case "o":
                    self.o_header, err = sippy_sdp.ParseSdpOrigin(v)
                    if err != nil {
                        return nil, err
                    }
                case "s":
                    self.s_header = sippy_sdp.ParseSdpGeneric(v)
                case "i":
                    self.i_header = sippy_sdp.ParseSdpGeneric(v)
                case "u":
                    self.u_header = sippy_sdp.ParseSdpGeneric(v)
                case "e":
                    self.e_header = sippy_sdp.ParseSdpGeneric(v)
                case "p":
                    self.p_header = sippy_sdp.ParseSdpGeneric(v)
                case "b":
                    self.b_header = sippy_sdp.ParseSdpGeneric(v)
                case "t":
                    self.t_header = sippy_sdp.ParseSdpGeneric(v)
                case "r":
                    self.r_header = sippy_sdp.ParseSdpGeneric(v)
                case "z":
                    self.z_header = sippy_sdp.ParseSdpGeneric(v)
                case "k":
                    self.k_header = sippy_sdp.ParseSdpGeneric(v)
                }
            }
        } else {
            self.sections[len(self.sections)-1].AddHeader(name, v)
        }
    }
    if c_header != nil {
        for _, section := range self.sections {
            if section.GetCHeader() == nil {
                section.SetCHeader(c_header)
            }
        }
        if len(self.sections) == 0 {
            self.c_header = c_header
        }
    }
    // Do some sanity checking, RFC4566
    switch {
    case self.v_header == nil:
        return nil, fmt.Errorf("Mandatory \"v=\" SDP header is missing")
    case self.o_header == nil:
        return nil, fmt.Errorf("Mandatory \"o=\" SDP header is missing")
    case self.s_header == nil:
        return nil, fmt.Errorf("Mandatory \"s=\" SDP header is missing")
    case self.t_header == nil:
        return nil, fmt.Errorf("Mandatory \"t=\" SDP header is missing")
    }
    for _, sect := range self.sections {
        if err := sect.SanityCheck(); err != nil {
            return nil, err
        }
    }
    return self, nil
}

func (self *sdpBody) first_half() []*sippy_sdp.Sdp_header_and_name {
    ret := []*sippy_sdp.Sdp_header_and_name{}
    if self.v_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "v", self.v_header }) }
    if self.o_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "o", self.o_header }) }
    if self.s_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "s", self.s_header }) }
    if self.i_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "i", self.i_header }) }
    if self.u_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "u", self.u_header }) }
    if self.e_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "e", self.e_header }) }
    if self.p_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "p", self.p_header }) }
    return ret
}

func (self *sdpBody) second_half() []*sippy_sdp.Sdp_header_and_name {
    ret := []*sippy_sdp.Sdp_header_and_name{}
    if self.b_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "b", self.b_header }) }
    if self.t_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "t", self.t_header }) }
    if self.r_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "r", self.r_header }) }
    if self.z_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "z", self.z_header }) }
    if self.k_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "k", self.k_header }) }
    return ret
}

func (self *sdpBody) all_headers() []*sippy_sdp.Sdp_header_and_name {
    ret := self.first_half()
    if self.c_header != nil { ret = append(ret, &sippy_sdp.Sdp_header_and_name{ "c", self.c_header }) }
    return append(ret, self.second_half()...)
}

func (self *sdpBody) String() string {
    s := ""
    if len(self.sections) == 1 && self.sections[0].GetCHeader() != nil {
        for _, it := range self.first_half() {
            s += it.Name + "=" + it.Header.String() + "\r\n"
        }
        s += "c=" + self.sections[0].GetCHeader().String() + "\r\n"
        for _, it := range self.second_half() {
            s += it.Name + "=" + it.Header.String() + "\r\n"
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
    if len(self.sections) > 1 && self.c_header == nil && self.sections[0].GetCHeader() != nil &&
      self.sections[0].GetCHeader().String() == self.sections[1].GetCHeader().String() {
        // Special code to optimize for the cases when there are many media streams pointing to
        // the same IP. Only include c= header into the top section of the SDP and remove it from
        // the streams that match.
        optimize_c_headers = true
        sections_0_str = self.sections[0].GetCHeader().String()
    }
    if optimize_c_headers {
        for _, it := range self.first_half() {
            s += it.Name + "=" + it.Header.String() + "\r\n"
        }
        s += "c=" + sections_0_str + "\r\n"
        for _, it := range self.second_half() {
            s += it.Name + "=" + it.Header.String() + "\r\n"
        }
    } else {
        for _, it := range self.all_headers() {
            s += it.Name + "=" + it.Header.String() + "\r\n"
        }
    }
    for _, header := range self.a_headers {
        s += "a=" + header + "\r\n"
    }
    for _, section := range self.sections {
        if optimize_c_headers && section.GetCHeader() != nil && section.GetCHeader().String() == sections_0_str {
            s += section.LocalStr(nil, true /* noC */)
        } else {
            s += section.String()
        }
    }
    return s
}

func (self *sdpBody) LocalStr(hostport *sippy_conf.HostPort) string {
    s := ""
    if len(self.sections) == 1 && self.sections[0].GetCHeader() != nil {
        for _, it := range self.first_half() {
            s += it.Name + "=" + it.Header.LocalStr(hostport) + "\r\n"
        }
        s += "c=" + self.sections[0].GetCHeader().String() + "\r\n"
        for _, it := range self.second_half() {
            s += it.Name + "=" + it.Header.LocalStr(hostport) + "\r\n"
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
    if len(self.sections) > 1 && self.c_header == nil && self.sections[0].GetCHeader() != nil &&
      self.sections[0].GetCHeader().String() == self.sections[1].GetCHeader().String() {
        // Special code to optimize for the cases when there are many media streams pointing to
        // the same IP. Only include c= header into the top section of the SDP and remove it from
        // the streams that match.
        optimize_c_headers = true
        sections_0_str = self.sections[0].GetCHeader().String()
    }
    if optimize_c_headers {
        for _, it := range self.first_half() {
            s += it.Name + "=" + it.Header.LocalStr(hostport) + "\r\n"
        }
        s += "c=" + sections_0_str + "\r\n"
        for _, it := range self.second_half() {
            s += it.Name + "=" + it.Header.LocalStr(hostport) + "\r\n"
        }
    } else {
        for _, it := range self.all_headers() {
            s += it.Name + "=" + it.Header.LocalStr(hostport) + "\r\n"
        }
    }
    for _, header := range self.a_headers {
        s += "a=" + header + "\r\n"
    }
    for _, section := range self.sections {
        if optimize_c_headers && section.GetCHeader() != nil &&
          section.GetCHeader().String() == sections_0_str {
            s += section.LocalStr(hostport, /*noC =*/ true)
        } else {
            s += section.LocalStr(hostport, /*noC =*/ false)
        }
    }
    return s
}

func (self *sdpBody) GetCopy() sippy_types.ParsedMsgBody {
    sections := make([]*sippy_sdp.SdpMediaDescription, len(self.sections))
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
        sect.GetCHeader().SetAddr(addr)
    }
}

func (self *sdpBody) GetSections() []*sippy_sdp.SdpMediaDescription {
    return self.sections
}

func (self *sdpBody) SetSections(sections []*sippy_sdp.SdpMediaDescription) {
    self.sections = sections
}

func (self *sdpBody) RemoveSection(idx int) {
    if idx < 0 || idx >= len(self.sections) {
        return
    }
    self.sections = append(self.sections[:idx], self.sections[idx + 1:]...)
}

func (self *sdpBody) SetOHeader(o_header *sippy_sdp.SdpOrigin) {
    self.o_header = o_header
}

func (self *sdpBody) AppendAHeader(hdr string) {
    self.a_headers = append(self.a_headers, hdr)
}
