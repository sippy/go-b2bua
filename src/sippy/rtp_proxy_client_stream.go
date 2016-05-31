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
    "fmt"
    "net"
    "strings"
    "syscall"
    "time"

    "sippy/conf"
    "sippy/math"
    "sippy/time"
)

const (
    _RTPPLWorker_MAX_RECURSE = 10
)

type rtpp_req_stream struct {
    command         string
    result_callback func(string)
}

type _RTPPLWorker struct {
    userv           *Rtp_proxy_client_stream
    s               net.Conn
    shutdown_chan   chan int
}

func NewRTPPLWorker(userv *Rtp_proxy_client_stream) *_RTPPLWorker {
    self := &_RTPPLWorker{
        userv           : userv,
        shutdown_chan   : make(chan int, 1),
    }
    self.connect()
    go self.run()
    return self
}

func (self *_RTPPLWorker) connect() {
    self.s, _ = net.Dial(self.userv.address.Network(), self.userv.address.String())
}

func (self *_RTPPLWorker) send_raw(command string, _recurse int, stime *sippy_time.MonoTime) (string, time.Duration, error) {
    if _recurse > _RTPPLWorker_MAX_RECURSE {
        return "", 0, fmt.Errorf("Cannot reconnect: " + self.userv.address.String())
    }
    if command[len(command)-1] != '\n' {
        command += "\n"
    }
    //print "%s.send_raw(%s)" % (id(self), command)
    if stime == nil {
        stime, _ = sippy_time.NewMonoTime()
    }
LOOP1:
    for {
        _, err := self.s.Write([]byte(command))
        switch err {
        case nil:
            break LOOP1
        case syscall.EINTR:
            continue
        case syscall.EPIPE: fallthrough
        case syscall.ENOTCONN: fallthrough
        case syscall.ECONNRESET:
            self.connect()
            return self.send_raw(command, _recurse + 1, stime)
        default:
            return "", 0, err
        }
    }
    buf := make([]byte, 1024)
    rval := ""
    for {
        n, err := self.s.Read(buf)
        switch err {
        case syscall.EINTR:
            continue
        case syscall.EPIPE: fallthrough
        case syscall.ENOTCONN: fallthrough
        case syscall.ECONNRESET:
            self.connect()
            return self.send_raw(command, _recurse + 1, stime)
        default:
            return "", 0, err
        }
        if n == 0 {
            self.connect()
            return self.send_raw(command, _RTPPLWorker_MAX_RECURSE, stime)
        }
        rval = strings.TrimSpace(string(buf[:n]))
        break
    }
    rtpc_delay, _ := stime.OffsetFromNow()
    return rval, rtpc_delay, nil
}

func (self *_RTPPLWorker) run() {
    for {
        req := <-self.userv.wi
        if req == nil {
            // Shutdown request, relay it further
            self.userv.wi <- nil
            break
        }
        //command, result_callback, callback_parameters = wi
        data, rtpc_delay, err := self.send_raw(req.command, 0, nil)
        if err != nil {
            println("Error communicating the rtpproxy: " + err.Error())
            data, rtpc_delay = "", -1
        }
        if len(data) == 0 {
            rtpc_delay = -1
        }
        if req.result_callback != nil {
            go req.result_callback(data)
        }
        if rtpc_delay != -1 {
            go self.userv.register_delay(rtpc_delay)
        }
    }
    self.shutdown_chan <- 1
}

type Rtp_proxy_client_stream struct {
    address     net.Addr
    nworkers    int
    workers     []*_RTPPLWorker
    delay_flt   sippy_math.RecFilter
    _is_local    bool
    wi          chan *rtpp_req_stream
}
/*
class Rtp_proxy_client_stream(object):
    wi_available = nil
    wi = nil
*/
func NewRtp_proxy_client_stream(owner *Rtp_proxy_client_base, global_config sippy_conf.Config, address net.Addr, opts *Rtp_proxy_opts) (rtp_proxy_transport, error) {
    var err error
    if address == nil {
        address, err = net.ResolveUnixAddr("unix", "/var/run/rtpproxy.sock")
        if err != nil {
            return nil, err
        }
    }
    nworkers := 1
    if opts != nil && opts.Nworkers != nil {
        nworkers = *opts.Nworkers
    }
    self := &Rtp_proxy_client_stream{
        address     : address,
        nworkers    : nworkers,
        workers     : make([]*_RTPPLWorker, nworkers),
        delay_flt   : sippy_math.NewRecFilter(0.95, 0.25),
        wi          : make(chan *rtpp_req_stream),
    }
    if strings.HasPrefix(address.Network(), "unix") {
        self._is_local = true
    } else {
        self._is_local = false
    }
    //self.wi_available = Condition()
    //self.wi = []
    for i := 0; i < self.nworkers; i++ {
        self.workers[i] = NewRTPPLWorker(self)
    }
    return self, nil
}

func (self *Rtp_proxy_client_stream) is_local() bool {
    return self._is_local
}

func (self *Rtp_proxy_client_stream) send_command(command string, result_callback func(string)) {
    if command[len(command)-1] != '\n' {
        command += "\n"
    }
    self.wi <- &rtpp_req_stream{command, result_callback }
}
/*
    def reconnect(self, address, bind_address = nil):
        self.shutdown()
        self.address = address
        self.workers = []
        for i in range(0, self.nworkers):
            self.workers.append(_RTPPLWorker(self))
        self.delay_flt = recfilter(0.95, 0.25)
*/
func (self *Rtp_proxy_client_stream) shutdown() {
    self.wi <- nil
    for _, rworker := range self.workers {
        <-rworker.shutdown_chan
    }
    self.workers = nil
}

func (self *Rtp_proxy_client_stream) register_delay(rtpc_delay time.Duration) {
    self.delay_flt.Apply(rtpc_delay.Seconds())
}
/*
    def get_rtpc_delay(self):
        return self.delay_flt.lastval

if __name__ == "__main__":
    from twisted.internet import reactor
    def display(*args):
        print args
        reactor.crash()
    r = Rtp_proxy_client_stream({"_sip_address":"1.2.3.4"})
    r.send_command("VF 123456", display, "abcd")
    reactor.run(installSignalHandlers = 1)
    r.shutdown()
*/
