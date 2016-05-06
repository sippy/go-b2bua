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
package sippy_sdp

import (
    "strings"
    "time"
    "strconv"
    "sync/atomic"

    "sippy/conf"
)

var _sdp_session_id int64

func init() {
    _sdp_session_id = time.Now().UnixNano() / 1000000
}

type SdpOrigin struct {
    username        string
    session_id      string
    version         string
    network_type    string
    address_type    string
    address         *sippy_conf.MyAddress
}

func ParseSdpOrigin(body string) *SdpOrigin {
    arr := strings.Fields(body)
    if len(arr) != 6 {
        return nil
    }
    return &SdpOrigin{
        username        : arr[0],
        session_id      : arr[1],
        version         : arr[2],
        network_type    : arr[3],
        address_type    : arr[4],
        address         : sippy_conf.NewMyAddress(arr[5]),
    }
}

func NewSdpOrigin(config sippy_conf.Config) *SdpOrigin {
    self := &SdpOrigin {
        username        : "-",
        session_id      : strconv.FormatInt(atomic.AddInt64(&_sdp_session_id, 1), 10),
        network_type    : "IN",
        address_type    : "IP4",
        address         : config.GetMyAddress(),
    }
    self.version = self.session_id
    return self
}

func (self *SdpOrigin) String() string {
    return strings.Join([]string{ self.username, self.session_id, self.version, self.network_type, self.address_type, self.address.String() }, " ")
}

func (self *SdpOrigin) LocalStr(hostport *sippy_conf.HostPort) string {
    if hostport != nil && self.address.IsSystemDefault() {
        var address_type string
        local_addr := hostport.Host.String()

        if local_addr[0] == '[' {
            address_type = "IP6"
            local_addr = local_addr[1:len(local_addr)-1]
        } else {
            address_type = "IP4"
        }
        return strings.Join([]string{ self.username, self.session_id, self.version, self.network_type, address_type, local_addr }, " ")
    }
    return strings.Join([]string{ self.username, self.session_id, self.version, self.network_type, self.address_type, self.address.String() }, " ")
}

func (self *SdpOrigin) GetCopy() *SdpOrigin {
    if self == nil {
        return nil
    }
    var ret SdpOrigin = *self
    return &ret
}
