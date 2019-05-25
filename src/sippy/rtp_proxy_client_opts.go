// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2014 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2016 Andriy Pylypenko. All rights reserved.
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
    "net"
    "strings"
    "time"

    "sippy/conf"
    "sippy/log"
    "sippy/net"
    "sippy/types"
)

type rtpProxyClientOpts struct {
    no_version_check    bool
    nworkers            *int
    hrtb_retr_ival      time.Duration
    hrtb_ival           time.Duration
    rtpp_class func(sippy_types.RtpProxyClient, sippy_conf.Config, net.Addr, *sippy_net.HostPort) (rtp_proxy_transport, error)
    rtppaddr            net.Addr
    config              sippy_conf.Config
    logger              sippy_log.ErrorLogger
    proxy_address       string
    bind_address        *sippy_net.HostPort
}

func NewRtpProxyClientOpts(spath string, bind_address *sippy_net.HostPort, config sippy_conf.Config, logger sippy_log.ErrorLogger) (*rtpProxyClientOpts, error) {
    self := &rtpProxyClientOpts{
        hrtb_retr_ival      : 60 * time.Second,
        hrtb_ival           : 10 * time.Second,
        no_version_check    : false,
        logger              : logger,
        config              : config,
        bind_address        : bind_address,
    }
    var err error

    if strings.HasPrefix(spath, "udp:") {
        tmp := strings.SplitN(spath, ":", 3)
        if len(tmp) == 2 {
            self.rtppaddr, err = net.ResolveUDPAddr("udp", tmp[1] + ":22222")
        } else {
            self.rtppaddr, err = net.ResolveUDPAddr("udp", tmp[1] + ":" + tmp[2])
        }
        if err != nil { return nil, err }
        self.proxy_address, _, err = net.SplitHostPort(self.rtppaddr.String())
        if err != nil { return nil, err }
        self.rtpp_class = newRtp_proxy_client_udp
    } else if strings.HasPrefix(spath, "udp6:") {
        tmp := strings.SplitN(spath, ":", 2)
        spath := tmp[1]
        rtp_proxy_host, rtp_proxy_port := spath, "22222"
        if spath[len(spath)-1] != ']' {
            idx := strings.LastIndexByte(spath, ':')
            if idx < 0 {
                rtp_proxy_host = spath
            } else {
                rtp_proxy_host, rtp_proxy_port = spath[:idx], spath[idx+1:]
            }
        }
        if rtp_proxy_host[0] != '[' {
            rtp_proxy_host = "[" + rtp_proxy_host + "]"
        }
        self.rtppaddr, err = net.ResolveUDPAddr("udp", rtp_proxy_host + ":" + rtp_proxy_port)
        if err != nil { return nil, err }
        self.proxy_address, _, err = net.SplitHostPort(self.rtppaddr.String())
        if err != nil { return nil, err }
        self.rtpp_class = newRtp_proxy_client_udp
    } else if strings.HasPrefix(spath, "tcp:") {
        tmp := strings.SplitN(spath, ":", 3)
        if len(tmp) == 2 {
            self.rtppaddr, err = net.ResolveTCPAddr("tcp", tmp[1] + ":22222")
        } else {
            self.rtppaddr, err = net.ResolveTCPAddr("tcp", tmp[1] + ":" + tmp[2])
        }
        if err != nil { return nil, err }
        self.proxy_address, _, err = net.SplitHostPort(self.rtppaddr.String())
        if err != nil { return nil, err }
        self.rtpp_class = newRtp_proxy_client_stream
    } else if strings.HasPrefix(spath, "tcp6:") {
        tmp := strings.SplitN(spath, ":", 2)
        spath := tmp[1]
        rtp_proxy_host, rtp_proxy_port := spath, "22222"
        if spath[len(spath)-1] != ']' {
            idx := strings.LastIndexByte(spath, ':')
            if idx < 0 {
                rtp_proxy_host = spath
            } else {
                rtp_proxy_host, rtp_proxy_port = spath[:idx], spath[idx+1:]
            }
        }
        if rtp_proxy_host[0] != '[' {
            rtp_proxy_host = "[" + rtp_proxy_host + "]"
        }
        self.rtppaddr, err = net.ResolveTCPAddr("tcp", rtp_proxy_host + ":" + rtp_proxy_port)
        if err != nil { return nil, err }
        self.proxy_address, _, err = net.SplitHostPort(self.rtppaddr.String())
        if err != nil { return nil, err }
        self.rtpp_class = newRtp_proxy_client_stream
    } else {
        if strings.HasPrefix(spath, "unix:") {
            self.rtppaddr, err = net.ResolveUnixAddr("unix", spath[5:])
        } else if strings.HasPrefix(spath, "cunix:") {
            self.rtppaddr, err = net.ResolveUnixAddr("unix", spath[6:])
        } else {
            self.rtppaddr, err = net.ResolveUnixAddr("unix", spath)
        }
        if err != nil { return nil, err }
        self.proxy_address = self.config.SipAddress().String()
        self.rtpp_class = newRtp_proxy_client_stream
    }
    return self, nil
}


func (self *rtpProxyClientOpts) SetHeartbeatInterval(ival time.Duration) {
    self.hrtb_ival = ival
}

func (self *rtpProxyClientOpts) SetHeartbeatRetryInterval(ival time.Duration) {
    self.hrtb_retr_ival = ival
}

func (self *rtpProxyClientOpts) GetNWorkers() *int {
    return self.nworkers
}
