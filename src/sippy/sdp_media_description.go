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
    "sippy/conf"
)

type sdpMediaDescription struct {
    m_header *sdpMedia
    i_header *sdpGeneric
    c_header *sdpConnecton
    b_header *sdpGeneric
    k_header *sdpGeneric
    a_headers []string
}

func (self *sdpMediaDescription) GetCopy() *sdpMediaDescription {
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

func NewSdpMediaDescription() *sdpMediaDescription {
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

func (self *sdpMediaDescription) addHeader(name, header string) {
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
