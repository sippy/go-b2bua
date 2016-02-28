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

type sdpMediaDescription struct {
    m_header *sdpMedia
    i_header *sdpGeneric
    c_header sippy_types.SdpConnecton
    b_header *sdpGeneric
    k_header *sdpGeneric
    a_headers []string
}

func (self *sdpMediaDescription) GetCopy() sippy_types.SdpMediaDescription {
    a_headers := make([]string, len(self.a_headers))
    copy(a_headers, self.a_headers)
    return &sdpMediaDescription{
        m_header : self.m_header.GetCopy(),
        i_header : self.i_header.GetCopy(),
        c_header : self.c_header.GetCopy(),
        b_header : self.b_header.GetCopy(),
        k_header : self.k_header.GetCopy(),
        a_headers : a_headers,
    }
}

func NewSdpMediaDescription() sippy_types.SdpMediaDescription {
    return &sdpMediaDescription{
        a_headers : make([]string, 0),
    }
}

func (self *sdpMediaDescription) all_headers() []*sdp_header_and_name {
    ret := []*sdp_header_and_name{}
    if self.m_header != nil { ret = append(ret, &sdp_header_and_name{ "m", self.m_header }) }
    if self.i_header != nil { ret = append(ret, &sdp_header_and_name{ "i", self.i_header }) }
    if self.c_header != nil { ret = append(ret, &sdp_header_and_name{ "c", self.c_header }) }
    if self.b_header != nil { ret = append(ret, &sdp_header_and_name{ "b", self.b_header }) }
    if self.k_header != nil { ret = append(ret, &sdp_header_and_name{ "k", self.k_header }) }
    return ret
}

func (self *sdpMediaDescription) String() string {
    s := ""
    for _, it := range self.all_headers() {
        s += it.name + "=" + it.header.String() + "\r\n"
    }
    for _, header := range self.a_headers {
        s += "a=" + header + "\r\n"
    }
    return s
}

func (self *sdpMediaDescription) LocalStr(hostport *sippy_conf.HostPort, noC bool) string {
    s := ""
    for _, it := range self.all_headers() {
        if noC && it.name == "c" {
            continue
        }
        s += it.name + "=" + it.header.LocalStr(hostport) + "\r\n"
    }
    for _, header := range self.a_headers {
        s += "a=" + header + "\r\n"
    }
    return s
}

func (self *sdpMediaDescription) AddHeader(name, header string) {
    if name == "a" {
        self.a_headers = append(self.a_headers, header)
    } else {
        switch name {
        case "m":
            self.m_header = ParseSdpMedia(header)
        case "i":
            self.i_header = ParseSdpGeneric(header)
        case "c":
            self.c_header = ParseSdpConnecton(header)
        case "b":
            self.b_header = ParseSdpGeneric(header)
        case "k":
            self.k_header = ParseSdpGeneric(header)
        }
    }
}

func (self *sdpMediaDescription) SetCHeaderAddr(addr string) {
    self.c_header.SetAddr(addr)
}

func (self *sdpMediaDescription) GetMHeader() sippy_types.SdpMedia {
    if self.m_header == nil {
        return nil
    }
    return self.m_header
}

func (self *sdpMediaDescription) GetCHeader() sippy_types.SdpConnecton {
    if self.c_header == nil {
        return nil
    }
    return self.c_header
}

func (self *sdpMediaDescription) SetCHeader(c_header sippy_types.SdpConnecton) {
    self.c_header = c_header
}

func (self *sdpMediaDescription) RemoveAHeader(hdr string) {
    new_a_hdrs := []string{}
    for _, h := range self.a_headers {
        if h == hdr {
            continue
        }
        new_a_hdrs = append(new_a_hdrs, h)
    }
    self.a_headers = new_a_hdrs
}

func (self *sdpMediaDescription) SetFormats(formats []string) {
    if self.m_header != nil {
        self.m_header.formats = formats
        self.optimize_a()
    }
}

func (self *sdpMediaDescription) optimize_a() {
    new_a_headers := []string{}
    for _, ah := range self.a_headers {
        pt := ""
        if strings.HasPrefix(ah, "rtpmap:") {
            pt = strings.Split(ah[7:], " ")[0]
        } else if strings.HasPrefix(ah, "fmtp:") {
            pt = strings.Split(ah[5:], " ")[0]
        }
        if pt != "" && ! self.m_header.HasFormat(pt) {
            continue
        }
        new_a_headers = append(new_a_headers, ah)
    }
    self.a_headers = new_a_headers
}
