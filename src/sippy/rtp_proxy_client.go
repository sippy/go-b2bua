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
    "bufio"
    "math/rand"
    "net"
    "time"
    "strconv"
    "strings"

    "sippy/conf"
    "sippy/log"
    "sippy/types"
)

func NewRtpProxyClient(opts *rtpProxyClientOpts, config sippy_conf.Config, logger sippy_log.ErrorLogger) (sippy_types.RtpProxyClient, error) {
    rtpp := NewRtp_proxy_client_base(nil, config, opts, logger)
    err := rtpp.Init()
    return rtpp, err
}

type Rtp_proxy_client_base struct {
    heir            sippy_types.RtpProxyClient
    opts            *rtpProxyClientOpts
    transport       rtp_proxy_transport
    proxy_address   string
    online          bool
    sbind_supported bool
    tnot_supported  bool
    copy_supported  bool
    stat_supported  bool
    wdnt_supported  bool
    caps_done       bool
    shut_down       bool
    active_sessions int64
    sessions_created int64
    active_streams  int64
    preceived       int64
    ptransmitted    int64
    _CAPSTABLE      []struct{ vers string; attr *bool }
    logger          sippy_log.ErrorLogger
    global_config   sippy_conf.Config
}

type rtp_proxy_transport interface {
    is_local() bool
    send_command(string, func(string))
    shutdown()
}

func (self *Rtp_proxy_client_base) IsLocal() bool {
    return self.transport.is_local()
}

func (self *Rtp_proxy_client_base) IsOnline() bool {
    return self.online
}

func (self *Rtp_proxy_client_base) SBindSupported() bool {
    return self.sbind_supported
}

func (self *Rtp_proxy_client_base) TNotSupported() bool {
    return self.tnot_supported
}

func (self *Rtp_proxy_client_base) GetProxyAddress() string {
    return self.proxy_address
}

func (self *Rtp_proxy_client_base) me() sippy_types.RtpProxyClient {
    if self.heir != nil {
        return self.heir
    }
    return self
}

func NewRtp_proxy_client_base(heir sippy_types.RtpProxyClient, global_config sippy_conf.Config, opts *rtpProxyClientOpts, logger sippy_log.ErrorLogger) (*Rtp_proxy_client_base) {
    if opts == nil {
        opts = NewRtpProxyClientOpts() // default settings
    }
    self := &Rtp_proxy_client_base{
        heir            : heir,
        caps_done       : false,
        shut_down       : false,
        logger          : logger,
        opts            : opts,
        global_config   : global_config,
    }
    self._CAPSTABLE = []struct{ vers string; attr *bool }{
        { "20071218", &self.copy_supported },
        { "20080403", &self.stat_supported },
        { "20081224", &self.tnot_supported },
        { "20090810", &self.sbind_supported },
        { "20150617", &self.wdnt_supported },
    }
    return self
}

func (self *Rtp_proxy_client_base) Init() error {
    var err error
    var rtpp_class func(sippy_types.RtpProxyClient, sippy_conf.Config, net.Addr) (rtp_proxy_transport, error)
    var rtppa net.Addr

    a := self.opts.spath
    if strings.HasPrefix(a, "udp:") {
        tmp := strings.SplitN(a, ":", 3)
        if len(tmp) == 2 {
            rtppa, err = net.ResolveUDPAddr("udp", tmp[1] + ":22222")
        } else {
            rtppa, err = net.ResolveUDPAddr("udp", tmp[1] + ":" + tmp[2])
        }
        if err != nil { return err }
        self.proxy_address, _, err = net.SplitHostPort(rtppa.String())
        if err != nil { return err }
        rtpp_class = newRtp_proxy_client_udp
    } else if strings.HasPrefix(a, "udp6:") {
        tmp := strings.SplitN(a, ":", 2)
        a := tmp[1]
        rtp_proxy_host, rtp_proxy_port := a, "22222"
        if a[len(a)-1] != ']' {
            idx := strings.LastIndexByte(a, ':')
            if idx < 0 {
                rtp_proxy_host = a
            } else {
                rtp_proxy_host, rtp_proxy_port = a[:idx], a[idx+1:]
            }
        }
        if rtp_proxy_host[0] != '[' {
            rtp_proxy_host = "[" + rtp_proxy_host + "]"
        }
        rtppa, err = net.ResolveUDPAddr("udp", rtp_proxy_host + ":" + rtp_proxy_port)
        if err != nil { return err }
        self.proxy_address, _, err = net.SplitHostPort(rtppa.String())
        if err != nil { return err }
        rtpp_class = newRtp_proxy_client_udp
    } else if strings.HasPrefix(a, "tcp:") {
        tmp := strings.SplitN(a, ":", 3)
        if len(tmp) == 2 {
            rtppa, err = net.ResolveTCPAddr("tcp", tmp[1] + ":22222")
        } else {
            rtppa, err = net.ResolveTCPAddr("tcp", tmp[1] + ":" + tmp[2])
        }
        if err != nil { return err }
        self.proxy_address, _, err = net.SplitHostPort(rtppa.String())
        if err != nil { return err }
        rtpp_class = newRtp_proxy_client_stream
    } else if strings.HasPrefix(a, "tcp6:") {
        tmp := strings.SplitN(a, ":", 2)
        a := tmp[1]
        rtp_proxy_host, rtp_proxy_port := a, "22222"
        if a[len(a)-1] != ']' {
            idx := strings.LastIndexByte(a, ':')
            if idx < 0 {
                rtp_proxy_host = a
            } else {
                rtp_proxy_host, rtp_proxy_port = a[:idx], a[idx+1:]
            }
        }
        if rtp_proxy_host[0] != '[' {
            rtp_proxy_host = "[" + rtp_proxy_host + "]"
        }
        rtppa, err = net.ResolveTCPAddr("tcp", rtp_proxy_host + ":" + rtp_proxy_port)
        if err != nil { return err }
        self.proxy_address, _, err = net.SplitHostPort(rtppa.String())
        if err != nil { return err }
        rtpp_class = newRtp_proxy_client_stream
    } else {
        if strings.HasPrefix(a, "unix:") {
            rtppa, err = net.ResolveUnixAddr("unix", a[5:])
        } else if strings.HasPrefix(a, "cunix:") {
            rtppa, err = net.ResolveUnixAddr("unix", a[6:])
        } else {
            rtppa, err = net.ResolveUnixAddr("unix", a)
        }
        self.proxy_address = self.global_config.SipAddress().String()
        rtpp_class = newRtp_proxy_client_stream
    }
    self.transport, err = rtpp_class(self.me(), self.global_config, rtppa)
    if err != nil {
        return err
    }
    self.start()
    return nil
}

func (self *Rtp_proxy_client_base) InitWithAddress(address net.Addr) error {
    var err error
    if strings.HasPrefix(address.Network(), "udp") {
        self.transport, err = newRtp_proxy_client_udp(self.me(), self.global_config, address)
        if err != nil {
            return err
        }
        self.proxy_address, _, err = net.SplitHostPort(address.String())
        if err != nil {
            return err
        }
    } else {
        self.transport, err = newRtp_proxy_client_stream(self.me(), self.global_config, address)
        if err != nil {
            return err
        }
        self.proxy_address = self.global_config.SipAddress().String()
    }
    self.start()
    return nil
}

func (self *Rtp_proxy_client_base) start() {
    if ! self.opts.no_version_check {
        self.version_check()
    } else {
        self.caps_done = true
        self.online = true
    }
}

func (self *Rtp_proxy_client_base) SendCommand(cmd string, cb func(string)) {
    self.transport.send_command(cmd, cb)
}
/*
    def reconnect(self, *args, **kwargs):
        self.rtpp_class.reconnect(self, *args, **kwargs)
*/

func (self *Rtp_proxy_client_base) version_check() {
    if self.shut_down {
        return
    }
    self.transport.send_command("V", self.version_check_reply)
}

func (self *Rtp_proxy_client_base) version_check_reply(version string) {
    if self.shut_down {
        return
    }
    if version == "20040107" {
        self.me().GoOnline()
    } else if self.online {
        self.me().GoOffline()
    } else {
        StartTimeout(self.version_check, nil, randomize(self.opts.hrtb_retr_ival, 0.1), 1, self.logger)
    }
}

func (self *Rtp_proxy_client_base) heartbeat() {
    //print "heartbeat", self, self.address
    if self.shut_down {
        return
    }
    self.transport.send_command("Ib", self.heartbeat_reply)
}

func (self *Rtp_proxy_client_base) heartbeat_reply(stats string) {
    //print "heartbeat_reply", self.address, stats, self.online
    if self.shut_down || ! self.online {
        return
    }
    if stats == "" {
        self.active_sessions = 0
        self.me().GoOffline()
    } else {
        sessions_created := int64(0)
        active_sessions := int64(0)
        active_streams := int64(0)
        preceived := int64(0)
        ptransmitted := int64(0)
        scanner := bufio.NewScanner(strings.NewReader(stats))
        for scanner.Scan() {
            line_parts := strings.SplitN(scanner.Text(), ":", 2)
            if len(line_parts) != 2 { continue }
            switch line_parts[0] {
            case "sessions created":
                sessions_created, _ = strconv.ParseInt(line_parts[1], 10, 64)
            case "active sessions":
                active_sessions, _ = strconv.ParseInt(line_parts[1], 10, 64)
            case "active streams":
                active_streams, _ = strconv.ParseInt(line_parts[1], 10, 64)
            case "packets received":
                preceived, _ = strconv.ParseInt(line_parts[1], 10, 64)
            case "packets transmitted":
                ptransmitted, _ = strconv.ParseInt(line_parts[1], 10, 64)
            }
        }
        self.update_active(active_sessions, sessions_created, active_streams, preceived, ptransmitted)
    }
    StartTimeout(self.heartbeat, nil, randomize(self.opts.hrtb_ival, 0.1), 1, self.logger)
}

func (self *Rtp_proxy_client_base) GoOnline() {
    if self.shut_down {
        return
    }
    if ! self.online {
        if ! self.caps_done {
            NewRtpp_caps_checker(self)
            return
        }
        self.online = true
        self.heartbeat()
    }
}

func (self *Rtp_proxy_client_base) GoOffline() {
    if self.shut_down {
        return
    }
    //print "go_offline", self.address, self.online
    if self.online {
        self.online = false
        StartTimeout(self.version_check, nil, randomize(self.opts.hrtb_retr_ival, 0.1), 1, self.logger)
    }
}

func (self *Rtp_proxy_client_base) update_active(active_sessions, sessions_created, active_streams, preceived, ptransmitted int64) {
    self.sessions_created = sessions_created
    self.active_sessions = active_sessions
    self.active_streams = active_streams
    self.preceived = preceived
    self.ptransmitted = ptransmitted
}

func (self *Rtp_proxy_client_base) Shutdown() {
    if self.shut_down { // do not crash when shutdown() called twice
        return
    }
    self.shut_down = true
    self.transport.shutdown()
    self.transport = nil
}

func (self *Rtp_proxy_client_base) GetOpts() sippy_types.RtpProxyClientOpts {
    return self.opts
}
/*
    def get_rtpc_delay(self):
        self.transport.get_rtpc_delay(self)
*/

type Rtpp_caps_checker struct {
    caps_requested  int
    caps_received   int
    rtpc            *Rtp_proxy_client_base
}

func NewRtpp_caps_checker(rtpc *Rtp_proxy_client_base) *Rtpp_caps_checker {
    self := &Rtpp_caps_checker{
        rtpc    : rtpc,
    }
    rtpc.caps_done = false
    self.caps_requested = len(rtpc._CAPSTABLE)
    for _, it := range rtpc._CAPSTABLE {
        attr := it.attr // For some reason the it.attr cannot be passed into the following
                        // function directly - the resulting value is always that of the
                        // last 'it.attr' value.
        rtpc.transport.send_command("VF " + it.vers, func(res string) { self.caps_query_done(res, attr) })
    }
    return self
}

func (self *Rtpp_caps_checker) caps_query_done(result string, attr *bool) {
    self.caps_received += 1
    if result == "1" {
        *attr = true
    } else {
        *attr = false
    }
    if self.caps_received == self.caps_requested {
        self.rtpc.caps_done = true
        self.rtpc.GoOnline()
        self.rtpc = nil
    }
}

func randomize(x time.Duration, p float64) time.Duration {
    return time.Duration(x.Seconds() * (1.0 + p * (1.0 - 2.0 * rand.Float64())) * float64(time.Second))
}
