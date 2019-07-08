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
    "crypto/rand"
    "errors"
    "net"
    "strings"
    "strconv"
    "sync/atomic"

    "sippy/net"
)

var _sdp_session_id int64

func init() {
    buf := make([]byte, 6)
    rand.Read(buf)
    for i := 0; i < len(buf); i++ {
        _sdp_session_id |= int64(buf[i]) << (uint(i) * 8)
    }
}

type SdpOrigin struct {
    username        string
    session_id      string
    version         int64
    network_type    string
    address_type    string
    address         string
}

func ParseSdpOrigin(body string) (*SdpOrigin, error) {
    arr := strings.Fields(body)
    if len(arr) != 6 {
        return nil, errors.New("Malformed field: " + body)
    }
    version, err := strconv.ParseInt(arr[2], 10, 64)
    if err != nil {
        return nil, err
    }
    return &SdpOrigin{
        username        : arr[0],
        session_id      : arr[1],
        version         : version,
        network_type    : arr[3],
        address_type    : arr[4],
        address         : arr[5],
    }, nil
}

func NewSdpOrigin() *SdpOrigin {
    // RFC4566
    // *******
    // For privacy reasons, it is sometimes desirable to obfuscate the
    // username and IP address of the session originator.  If this is a
    // concern, an arbitrary <username> and private <unicast-address> MAY be
    // chosen to populate the "o=" field, provided that these are selected
    // in a manner that does not affect the global uniqueness of the field.
    // *******
    sid := atomic.AddInt64(&_sdp_session_id, 1)
    return &SdpOrigin {
        username        : "-",
        session_id      : strconv.FormatInt(sid, 10),
        network_type    : "IN",
        address_type    : "IP4",
        address         : "192.0.2.1", // 192.0.2.0/24 (TEST-NET-1)
        version         : sid,
    }
}

func NewSdpOriginWithAddress(address string) (*SdpOrigin, error) {
    ip := net.ParseIP(address)
    if ip == nil {
        return nil, errors.New("The address is not IP address: " + address)
    }
    address_type := "IP4"
    if ! sippy_net.IsIP4(ip) {
        address_type = "IP6"
    }
    sid := atomic.AddInt64(&_sdp_session_id, 1)
    self := &SdpOrigin {
        username        : "-",
        session_id      : strconv.FormatInt(sid, 10),
        network_type    : "IN",
        address_type    : address_type,
        address         : address,
        version         : sid,
    }
    return self, nil
}

func (self *SdpOrigin) String() string {
    version := strconv.FormatInt(self.version, 10)
    return strings.Join([]string{ self.username, self.session_id, version, self.network_type, self.address_type, self.address }, " ")
}

func (self *SdpOrigin) LocalStr(hostport *sippy_net.HostPort) string {
    version := strconv.FormatInt(self.version, 10)
    return strings.Join([]string{ self.username, self.session_id, version, self.network_type, self.address_type, self.address }, " ")
}

func (self *SdpOrigin) GetCopy() *SdpOrigin {
    if self == nil {
        return nil
    }
    var ret SdpOrigin = *self
    return &ret
}

func (self *SdpOrigin) IncVersion() {
    self.version++
}

func (self *SdpOrigin) GetSessionId() string {
    return self.session_id
}

func (self *SdpOrigin) GetVersion() int64 {
    return self.version
}

func (self *SdpOrigin) SetAddress(addr string) {
    self.address = addr
}

func (self *SdpOrigin) SetAddressType(t string) {
    self.address_type = t
}

func (self *SdpOrigin) SetNetworkType(t string) {
    self.network_type = t
}
