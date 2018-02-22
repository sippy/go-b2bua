//
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

package sippy_conf

import (
    "net"
    "os"

    "sippy/log"
    "sippy/net"
)

type Config interface {
    SipAddress()    *sippy_net.MyAddress
    SipPort()       *sippy_net.MyPort
    GetIPV6Enabled()   bool
    SetIPV6Enabled(bool)
    SetSipAddress(*sippy_net.MyAddress)
    SetSipPort(*sippy_net.MyPort)
    SipLogger() sippy_log.SipLogger
    ErrorLogger() sippy_log.ErrorLogger
    GetMyUAName() string
    SetMyUAName(s string)
    SetAllowFormats(f []int)

    GetMyAddress() *sippy_net.MyAddress
    SetMyAddress(*sippy_net.MyAddress)
    GetMyPort() *sippy_net.MyPort
    SetMyPort(*sippy_net.MyPort)

    AutoConvertTelUrl() bool
    SetAutoConvertTelUrl(bool)
    GetSipTransportFactory() sippy_net.SipTransportFactory
    SetSipTransportFactory(sippy_net.SipTransportFactory)
}

type config struct {
    sip_address     *sippy_net.MyAddress
    sip_port        *sippy_net.MyPort
    sip_logger      sippy_log.SipLogger
    error_logger    sippy_log.ErrorLogger
    ipv6_enabled    bool

    my_address      *sippy_net.MyAddress
    my_port         *sippy_net.MyPort
    my_uaname       string
    allow_formats   []int
    autoconvert_tel_url bool
    tfactory        sippy_net.SipTransportFactory
}

func NewConfig(error_logger sippy_log.ErrorLogger, sip_logger sippy_log.SipLogger) Config {
    address := "127.0.0.1"
    if hostname, err := os.Hostname(); err == nil {
        if addresses, err := net.LookupHost(hostname); err == nil && len(addresses) > 0 {
            address = addresses[0]
        }
    }
    return &config{
        ipv6_enabled    : true,
        error_logger    : error_logger,
        sip_logger      : sip_logger,
        my_address  : sippy_net.NewSystemAddress(address),
        my_port     : sippy_net.NewSystemPort("5060"),
        my_uaname   : "Sippy",
        allow_formats : make([]int, 0),
        autoconvert_tel_url : false,
    }
}

func (self *config) SipAddress() *sippy_net.MyAddress {
    if self.sip_address == nil {
        return self.my_address
    }
    return self.sip_address
}

func (self *config) SipLogger() sippy_log.SipLogger {
    return self.sip_logger
}

func (self *config) SipPort() *sippy_net.MyPort {
    if self.sip_port == nil {
        return self.my_port
    }
    return self.sip_port
}

func (self *config) SetIPV6Enabled(v bool) {
    self.ipv6_enabled = v
}

func (self *config) GetIPV6Enabled() bool {
    return self.ipv6_enabled
}

func (self *config) ErrorLogger() sippy_log.ErrorLogger {
    return self.error_logger
}

func (self *config) SetSipAddress(addr *sippy_net.MyAddress) {
    self.sip_address = addr
}

func (self *config) SetSipPort(port *sippy_net.MyPort) {
    self.sip_port = port
}

func (self *config) GetMyAddress() (*sippy_net.MyAddress) {
    return self.my_address
}

func (self *config) SetMyAddress(addr *sippy_net.MyAddress) {
    self.my_address = addr
}

func (self *config) GetMyPort() (*sippy_net.MyPort) {
    return self.my_port
}

func (self *config) SetMyPort(port *sippy_net.MyPort) {
    self.my_port = port
}

func (self *config) GetMyUAName() string {
    return self.my_uaname
}

func (self *config) SetMyUAName(s string) {
    self.my_uaname = s
}

func (self *config) SetAllowFormats(f []int) {
    self.allow_formats = f
}

func (self *config) AutoConvertTelUrl() bool {
    return self.autoconvert_tel_url
}

func (self *config) SetAutoConvertTelUrl(v bool) {
    self.autoconvert_tel_url = v
}

func (self *config) GetSipTransportFactory() sippy_net.SipTransportFactory {
    return self.tfactory
}

func (self *config) SetSipTransportFactory(tfactory sippy_net.SipTransportFactory) {
    self.tfactory = tfactory
}
