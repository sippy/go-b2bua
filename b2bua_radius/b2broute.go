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
)

type B2BRoute struct {
    cld             string
    cld_set         bool
    hostport        string
    hostonly        string
    huntstop_scodes []int
    ainfo           []net.IP
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
    outbound_proxy  *sippy_conf.HostPort
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
    var port *sippy_conf.MyPort
    if len(hostport) == 1 {
        port = global_config.GetMyPort()
    } else {
        port = sippy_conf.NewMyPort(hostport[1])
    }
    self.ainfo, err = net.LookupIP(hostport[0])
    if ipv6only {
        // get rid of IPv4 addresses
        tmp := []net.IP{}
        for _, ip := range self.ainfo {
            if ip.To4() != nil { continue }
            tmp = append(tmp, ip)
        }
        self.ainfo = tmp
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
                ash, err = sippy.ParseSipHeader(v)
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
                self.outbound_proxy = sippy_conf.NewHostPort(av[1], "5060")
            } else {
                self.outbound_proxy = sippy_conf.NewHostPort(host_port[0], host_port[1])
            }
        //default:
        //    self.params[a] = v
        }
    }
    return self, nil
}
/*
    def customize(self, rnum, default_cld, default_cli, default_credit_time, \
      pass_headers, max_credit_time):
        self.rnum = rnum
        if ! self.cld_set:
            self.cld = default_cld
        if ! self.cli_set:
            self.cli = default_cli
        if ! self.crt_set:
            self.crt_set = default_credit_time
        if self.params.has_key("gt"):
            timeout, skip = self.params["gt"].split(",", 1)
            self.params["group_timeout"] = (int(timeout), rnum + int(skip))
        if self.extra_headers != nil:
            self.extra_headers = self.extra_headers + tuple(pass_headers)
        else:
            self.extra_headers = tuple(pass_headers)
        if max_credit_time != nil:
            if self.credit_time == nil or self.credit_time > max_credit_time:
                self.credit_time = max_credit_time

    def getCopy(self):
        if cself != nil:
            self.rnum = cself.rnum
            self.addrinfo = cself.addrinfo
            self.cld = cself.cld
            self.cld_set = cself.cld_set
            self.hostport = cself.hostport
            self.hostonly = cself.hostonly
            self.credit_time = cself.credit_time
            self.crt_set = cself.crt_set
            self.expires = cself.expires
            self.no_progress_expires = cself.no_progress_expires
            self.forward_on_fail = cself.forward_on_fail
            self.user = cself.user
            self.passw = cself.passw
            self.cli = cself.cli
            self.cli_set = cself.cli_set
            self.params = dict(cself.params)
            self.ainfo = cself.ainfo
            if cself.extra_headers != nil:
                self.extra_headers = tuple([x.getCopy() for x in cself.extra_headers])
            return
        return self.__class__(cself = self)

    def getNHAddr(self, source):
        if source[0].startswith("["):
            af = AF_INET6
        else:
            af = AF_INET
        amatch = [x[4] for x in self.ainfo if x[0] == af]
        same_af = true
        if len(amatch) == 0:
            same_af = false
            amatch = self.ainfo[0][4]
            af = self.ainfo[0][0]
        else:
            amatch = amatch[0]
        if af == AF_INET6:
            return ((("[%s]" % amatch[0], amatch[1]), same_af))
        return (((amatch[0], amatch[1]), same_af))
*/
