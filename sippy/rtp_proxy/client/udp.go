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

package rtp_proxy_client

import (
    "crypto/rand"
    "encoding/hex"
    "errors"
    "net"
    "time"
    "strings"
    "sync"

    "github.com/sippy/go-b2bua/sippy"
    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/fmt"
    "github.com/sippy/go-b2bua/sippy/math"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/utils"
    "github.com/sippy/go-b2bua/sippy/rtp_proxy/types"
)

type Rtp_proxy_client_udp struct {
    _address            net.Addr
    uopts               *sippy.UdpServerOpts
    pending_requests    map[string]*rtpp_req_udp
    global_config       sippy_conf.Config
    delay_flt           sippy_math.RecFilter
    worker              *sippy.UdpServer
    hostport            *sippy_net.HostPort
    bind_address        *sippy_net.HostPort
    lock                sync.Mutex
    owner               sippy_types.RtpProxyClient
}

type rtpp_req_udp struct {
    next_retr       float64
    triesleft       int64
    timer           *sippy.Timeout
    command         string
    result_callback func(string)
    stime           *sippy_time.MonoTime
    retransmits     int
}

func new_rtpp_req_udp(next_retr float64, triesleft int64, timer *sippy.Timeout, command string, result_callback func(string)) *rtpp_req_udp {
    stime, _ := sippy_time.NewMonoTime()
    return &rtpp_req_udp{
        next_retr       : next_retr,
        triesleft       : triesleft,
        timer           : timer,
        command         : command,
        result_callback : result_callback,
        stime           : stime,
        retransmits     : 0,
    }
}

func getnretrans(first_retr, timeout float64) (int64, error) {
    if first_retr < 0 {
        return 0, errors.New(sippy_fmt.Sprintf("getnretrans(%f, %f)", first_retr, timeout))
    }
    var n int64 = 0
    for {
        timeout -= first_retr
        if timeout < 0 {
            break
        }
        first_retr *= 2.0
        n += 1
    }
    return n, nil
}

func NewRtp_proxy_client_udp(owner sippy_types.RtpProxyClient, global_config sippy_conf.Config, address net.Addr, bind_address *sippy_net.HostPort) (rtp_proxy_types.RtpProxyTransport, error) {
    self := &Rtp_proxy_client_udp{
        owner               : owner,
        pending_requests    : make(map[string]*rtpp_req_udp),
        global_config       : global_config,
        delay_flt           : sippy_math.NewRecFilter(0.95, 0.25),
        bind_address        : bind_address,
    }
    laddress, err := self._setup_addr(address, bind_address)
    if err != nil {
        return nil, err
    }
    self.uopts = sippy.NewUdpServerOpts(laddress, self.process_reply)
    //self.uopts.ploss_out_rate = self.ploss_out_rate
    //self.uopts.pdelay_out_max = self.pdelay_out_max
    if owner.GetOpts().GetNWorkers() != nil {
        self.uopts.NWorkers = *owner.GetOpts().GetNWorkers()
    }
    self.worker, err = sippy.NewUdpServer(global_config, self.uopts)
    return self, err
}


func (self *Rtp_proxy_client_udp) _setup_addr(address net.Addr, bind_address *sippy_net.HostPort) (*sippy_net.HostPort, error) {
    var err error

    self.hostport, err = sippy_net.NewHostPortFromAddr(address)
    if err != nil {
        return nil, err
    }
    self._address = address
    self.bind_address = bind_address

    if bind_address == nil {
        if self.hostport.Host.String()[0] == '[' {
            if self.hostport.Host.String() == "[::1]" {
                bind_address = sippy_net.NewHostPort("[::1]", "0")
            } else {
                bind_address = sippy_net.NewHostPort("[::]", "0")
            }
        } else {
            if strings.HasPrefix(self.hostport.Host.String(), "127.") {
                bind_address = sippy_net.NewHostPort("127.0.0.1", "0")
            } else {
                bind_address = sippy_net.NewHostPort("0.0.0.0", "0")
            }
        }
    }
    return bind_address, nil
}

func (*Rtp_proxy_client_udp) Is_local() bool {
    return false
}

func (self *Rtp_proxy_client_udp) Address() net.Addr {
    return self._address
}

func (self *Rtp_proxy_client_udp) Send_command(command string, result_callback func(string)) {
    buf := make([]byte, 16)
    rand.Read(buf)
    cookie := hex.EncodeToString(buf)
    next_retr := self.delay_flt.GetLastval() * 4.0
    exp_time := 3.0
    if command[0] == 'I' {
        exp_time = 10.0
    } else if command[0] == 'G' {
        exp_time = 1.0
    }
    nretr, err := getnretrans(next_retr, exp_time)
    if err != nil {
        self.global_config.ErrorLogger().Debug("getnretrans error: " + err.Error())
        return
    }
    command = cookie + " " + command
    timer := sippy.StartTimeout(func() { self.retransmit(cookie) }, nil, time.Duration(next_retr * float64(time.Second)), 1, self.global_config.ErrorLogger())
    preq := new_rtpp_req_udp(next_retr, nretr - 1, timer, command, result_callback)
    self.worker.SendTo([]byte(command), self.hostport)
    self.lock.Lock()
    self.pending_requests[cookie] = preq
    self.lock.Unlock()
}

func (self *Rtp_proxy_client_udp) retransmit(cookie string) {
    self.lock.Lock()
    req, ok := self.pending_requests[cookie]
    if ! ok {
        self.lock.Unlock()
        return
    }
    if req.triesleft <= 0 || self.worker == nil {
        delete(self.pending_requests, cookie)
        self.lock.Unlock()
        self.owner.GoOffline()
        if req.result_callback != nil {
            sippy_utils.SafeCall(func() { req.result_callback("") }, nil/*lock*/, self.global_config.ErrorLogger())
        }
        return
    }
    self.lock.Unlock()
    req.next_retr *= 2
    req.retransmits += 1
    req.timer = sippy.StartTimeout(func() { self.retransmit(cookie) }, nil, time.Duration(req.next_retr * float64(time.Second)), 1, self.global_config.ErrorLogger())
    req.stime, _ = sippy_time.NewMonoTime()
    self.worker.SendTo([]byte(req.command), self.hostport)
    req.triesleft -= 1
}

func (self *Rtp_proxy_client_udp) process_reply(data []byte, address *sippy_net.HostPort, worker sippy_net.Transport, rtime *sippy_time.MonoTime) {
    arr := sippy_utils.FieldsN(string(data), 2)
    if len(arr) != 2 {
        self.global_config.ErrorLogger().Debug("Rtp_proxy_client_udp.process_reply(): invalid response " + string(data))
        return
    }
    cookie, result := arr[0], arr[1]
    self.lock.Lock()
    req, ok := self.pending_requests[cookie]
    delete(self.pending_requests, cookie)
    self.lock.Unlock()
    if ! ok {
        return
    }
    req.timer.Cancel()
    if req.result_callback != nil {
        sippy_utils.SafeCall(func() { req.result_callback(strings.TrimSpace(result)) }, nil/*lock*/, self.global_config.ErrorLogger())
    }
    if req.retransmits == 0 {
        // When we had to do retransmit it is not possible to figure out whether
        // or not this reply is related to the original request or one of the
        // retransmits. Therefore, using it to estimate delay could easily produce
        // bogus value that is too low or even negative if we cook up retransmit
        // while the original response is already in the queue waiting to be
        // processed. This should not be a big issue since UDP command channel does
        // not work very well if the packet loss goes to more than 30-40%.
        self.delay_flt.Apply(rtime.Sub(req.stime).Seconds())
        //print "Rtp_proxy_client_udp.process_reply(): delay %f" % (rtime - stime)
    }
}

func (self *Rtp_proxy_client_udp) Reconnect(address net.Addr, bind_address *sippy_net.HostPort) {
    if self._address.String() != address.String() || bind_address.String() != self.bind_address.String() {
        self.uopts.LAddress, _ = self._setup_addr(address, bind_address)
        self.worker.Shutdown()
        self.worker, _ = sippy.NewUdpServer(self.global_config, self.uopts)
        self.delay_flt = sippy_math.NewRecFilter(0.95, 0.25)
    }
}

func (self *Rtp_proxy_client_udp) Shutdown() {
    if self.worker == nil {
        return
    }
    self.worker.Shutdown()
    self.worker = nil
}

func (self *Rtp_proxy_client_udp) Get_rtpc_delay() float64 {
    return self.delay_flt.GetLastval()
}

/*
class selftest(object):
    def gotreply(self, *args):
        from twisted.internet import reactor
        print args
        reactor.crash()

    def run(self):
        import os
        from twisted.internet import reactor
        global_config = {}
        global_config["my_pid"] = os.getpid()
        rtpc = Rtp_proxy_client_udp(global_config, ("127.0.0.1", 22226), nil)
        os.system("sockstat | grep -w %d" % global_config["my_pid"])
        rtpc.send_command("Ib", self.gotreply)
        reactor.run()
        rtpc.reconnect(("localhost", 22226), ("0.0.0.0", 34222))
        os.system("sockstat | grep -w %d" % global_config["my_pid"])
        rtpc.send_command("V", self.gotreply)
        reactor.run()
        rtpc.reconnect(("localhost", 22226), ("127.0.0.1", 57535))
        os.system("sockstat | grep -w %d" % global_config["my_pid"])
        rtpc.send_command("V", self.gotreply)
        reactor.run()
        rtpc.shutdown()

if __name__ == "__main__":
    selftest().run()
*/
