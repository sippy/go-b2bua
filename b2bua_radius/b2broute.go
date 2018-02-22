// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2016 Sippy Software, Inc. All rights reserved.
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
package main

import (
    "errors"
    "net"
    "net/url"
    "strconv"
    "strings"
    "time"

    "sippy"
    "sippy/conf"
    "sippy/headers"
    "sippy/net"
)

type ainfo_item struct {
    ip          net.IP
    port        string
}

func (self *ainfo_item) HostPort() *sippy_net.HostPort {
    return sippy_net.NewHostPort(self.ip.String(), self.port)
}

type B2BRoute struct {
    cld             string
    cld_set         bool
    hostport        string
    hostonly        string
    huntstop_scodes []int
    ainfo           []*ainfo_item
    credit_time     time.Duration
    crt_set         bool
    expires         time.Duration
    no_progress_expires time.Duration
    forward_on_fail bool
    user            string
    passw           string
    cli             string
    cli_set         bool
    caller_name     string
    extra_headers   []sippy_header.SipHeader
    rtpp            bool
    outbound_proxy  *sippy_net.HostPort
    rnum            int
}
/*
from sippy.SipHeader import SipHeader
from sippy.SipConf import SipConf

from urllib import unquote
from socket import getaddrinfo, SOCK_STREAM, AF_INET, AF_INET6

class B2BRoute(object):
    rnum = nil
    addrinfo = nil
    params = nil
    ainfo = nil
*/

func NewB2BRoute(sroute string, global_config sippy_conf.Config) (*B2BRoute, error) {
    var hostport []string
    var err error

    self := &B2BRoute{
        huntstop_scodes : []int{},
        cld_set         : false,
        crt_set         : false,
        forward_on_fail : false,
        cli_set         : false,
        extra_headers   : []sippy_header.SipHeader{},
        rtpp            : true,
    }
    route := strings.Split(sroute, ";")
    if strings.IndexRune(route[0], '@') != -1 {
        tmp := strings.SplitN(route[0], "@", 2)
        self.cld, self.hostport = tmp[0], tmp[1]
        // Allow CLD to be forcefully removed by sending `Routing:@host" entry,
        // as opposed to the Routing:host, which means that CLD should be obtained
        // from the incoming call leg.
        self.cld_set = true
    } else {
        self.hostport = route[0]
    }
    ipv6only := false
    if self.hostport[0] != '[' {
        hostport = strings.SplitN(self.hostport, ":", 2)
        self.hostonly = hostport[0]
    } else {
        hostport = strings.SplitN(self.hostport[1:], "]", 2)
        if len(hostport) > 1 {
            if hostport[1] == "" {
                hostport = hostport[:1]
            } else {
                hostport[1] = hostport[1][1:]
            }
        }
        ipv6only = true
        self.hostonly = "[" + hostport[0] + "]"
    }
    var port *sippy_net.MyPort
    if len(hostport) == 1 {
        port = global_config.GetMyPort()
    } else {
        port = sippy_net.NewMyPort(hostport[1])
    }
    self.ainfo = make([]*ainfo_item, 0)
    ips, err := net.LookupIP(hostport[0])
    if err != nil {
        return nil, errors.New("NewB2BRoute: error resolving host IP '" + hostport[0] + "': " + err.Error())
    }
    for _, ip := range ips {
        if ipv6only && ip.To4() != nil {
            continue
        }
        self.ainfo = append(self.ainfo, &ainfo_item{ ip, port.String() })
    }
    //self.params = []string{}
    for _, x := range route[1:] {
        av := strings.SplitN(x, "=", 2)
        switch av[0] {
        case "credit-time":
            v, err := strconv.Atoi(av[1])
            if err != nil {
                return nil, errors.New("Error parsing credit-time '" + av[1] + "': " + err.Error())
            }
            if v < 0 { v = 0 }
            self.credit_time = time.Duration(v * int(time.Second))
            self.crt_set = true
        case "expires":
            v, err := strconv.Atoi(av[1])
            if err != nil {
                return nil, errors.New("Error parsing the expires '" + av[1] + "': " + err.Error())
            }
            if v < 0 { v = 0 }
            self.expires = time.Duration(v * int(time.Second))
        case "hs_scodes":
            for _, s := range strings.Split(av[1], ",") {
                s = strings.TrimSpace(s)
                if s == "" { continue }
                scode, err := strconv.Atoi(s)
                if err != nil {
                    return nil, errors.New("Error parsing hs_scodes '" + s + "': " + err.Error())
                }
                self.huntstop_scodes = append(self.huntstop_scodes, scode)
            }
        case "np_expires":
            v, err := strconv.Atoi(av[1])
            if err != nil {
                return nil, errors.New("Error parsing the no_progress_expires '" + av[1] + "': " + err.Error())
            }
            if v < 0 { v = 0 }
            self.no_progress_expires = time.Duration(v * int(time.Second))
        case "forward_on_fail":
            self.forward_on_fail = true
        case "auth":
            tmp := strings.SplitN(av[1], ":", 2)
            if len(tmp) != 2 {
                return nil, errors.New("Error parsing the auth (no colon) '" + av[1] + "': " + err.Error())
            }
            self.user, self.passw = tmp[0], tmp[1]
        case "cli":
            self.cli = av[1]
            self.cli_set = true
        case "cnam":
            self.caller_name, err = url.QueryUnescape(av[1])
            if err != nil {
                return nil, errors.New("Error parsing the cnam '" + av[1] + "': " + err.Error())
            }
        case "ash":
            var v string
            var ash []sippy_header.SipHeader
            v, err = url.QueryUnescape(av[1])
            if err == nil {
                ash, err = sippy.ParseSipHeader(v, global_config)
            }
            if err != nil {
                return nil, errors.New("Error parsing the ash '" + av[1] + "': " + err.Error())
            }
            self.extra_headers = append(self.extra_headers, ash...)
        case "rtpp":
            v, err := strconv.Atoi(av[1])
            if err != nil {
                return nil, errors.New("Error parsing the rtpp '" + av[1] + "': " + err.Error())
            }
            self.rtpp = (v != 0)
        case "op":
            host_port := strings.SplitN(av[1], ":", 2)
            if len(host_port) == 1 {
                self.outbound_proxy = sippy_net.NewHostPort(av[1], "5060")
            } else {
                self.outbound_proxy = sippy_net.NewHostPort(host_port[0], host_port[1])
            }
        //default:
        //    self.params[a] = v
        }
    }
    return self, nil
}

func (self *B2BRoute) customize(rnum int, default_cld, default_cli string, default_credit_time time.Duration, pass_headers []sippy_header.SipHeader, max_credit_time time.Duration) {
    self.rnum = rnum
    if ! self.cld_set {
        self.cld = default_cld
    }
    if ! self.cli_set {
        self.cli = default_cli
    }
    if ! self.crt_set {
        self.credit_time = default_credit_time
    }
    //if self.params.has_key("gt") {
    //    timeout, skip = self.params["gt"].split(",", 1)
    //    self.params["group_timeout"] = (int(timeout), rnum + int(skip))
    //}
    self.extra_headers = append(self.extra_headers, pass_headers...)
    if max_credit_time != 0 {
        if self.credit_time == 0 || self.credit_time > max_credit_time {
            self.credit_time = max_credit_time
        }
    }
}

func (self *B2BRoute) getCopy() *B2BRoute {
    if self == nil {
        return nil
    }
    cself := *self
    if self.outbound_proxy != nil {
        cself.outbound_proxy = self.outbound_proxy.GetCopy()
    }

    cself.huntstop_scodes = make([]int, len(self.huntstop_scodes))
    copy(cself.huntstop_scodes, self.huntstop_scodes)

    cself.ainfo = make([]*ainfo_item, len(self.ainfo))
    copy(cself.ainfo, self.ainfo)

    cself.extra_headers = make([]sippy_header.SipHeader, len(self.extra_headers))
    copy(cself.extra_headers, self.extra_headers)

    return &cself
}

func (self *B2BRoute) getNHAddr(source *sippy_net.HostPort) (*sippy_net.HostPort, bool) {
    src_ip := net.ParseIP(source.Host.String())
    if src_ip == nil {
        return self.ainfo[0].HostPort(), true
    }
    src_is_ipv4 := true
    if src_ip.To4() == nil {
        src_is_ipv4 = false
    }
    for _, it := range self.ainfo {
        if src_is_ipv4 && it.ip.To4() != nil {
            return it.HostPort(), true
        } else if ! src_is_ipv4 && it.ip.To4() == nil {
            return it.HostPort(), true
        }
    }
    return self.ainfo[0].HostPort(), true
}
