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
    "time"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/math"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/utils"
)

const (
    _RTPPLWorker_MAX_RETRIES = 3
)

type rtpp_req_stream struct {
    command         string
    result_callback func(string)
}

type _RTPPLWorker struct {
    userv           *Rtp_proxy_client_stream
    shutdown_chan   chan int
}

func newRTPPLWorker(userv *Rtp_proxy_client_stream) *_RTPPLWorker {
    self := &_RTPPLWorker{
        userv           : userv,
        shutdown_chan   : make(chan int, 1),
    }
    go self.run()
    return self
}

func (self *_RTPPLWorker) send_raw(command string, stime *sippy_time.MonoTime) (string, time.Duration, error) {
    //print "%s.send_raw(%s)" % (id(self), command)
    if stime == nil {
        stime, _ = sippy_time.NewMonoTime()
    }
    var err error
    var n int
    retries := 0
    rval := ""
    buf := make([]byte, 1024)
    var s net.Conn
    for {
        if retries > _RTPPLWorker_MAX_RETRIES {
            return "", 0, fmt.Errorf("Error sending to the rtpproxy on " + self.userv._address.String() + ": " + err.Error())
        }
        retries++
        if s != nil {
            s.Close()
        }
        s, err = net.Dial(self.userv._address.Network(), self.userv._address.String())
        if err != nil {
            time.Sleep(100 * time.Millisecond)
            continue
        }
        _, err = s.Write([]byte(command))
        if err != nil {
            time.Sleep(100 * time.Millisecond)
            continue
        }
        n, err = s.Read(buf)
        if err != nil {
            time.Sleep(100 * time.Millisecond)
            continue
        }
        rval = strings.TrimSpace(string(buf[:n]))
        break
    }
    s.Close()
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
        data, rtpc_delay, err := self.send_raw(req.command, nil)
        if err != nil {
            self.userv.global_config.ErrorLogger().Debug("Error communicating the rtpproxy: " + err.Error())
            data, rtpc_delay = "", -1
        }
        if len(data) == 0 {
            rtpc_delay = -1
        }
        if req.result_callback != nil {
            sippy_utils.SafeCall(func() { req.result_callback(data) }, nil/*lock*/, self.userv.global_config.ErrorLogger())
        }
        if rtpc_delay != -1 {
            self.userv.register_delay(rtpc_delay)
        }
    }
    self.shutdown_chan <- 1
    self.userv = nil
}

type Rtp_proxy_client_stream struct {
    owner       sippy_types.RtpProxyClient
    _address    net.Addr
    nworkers    int
    workers     []*_RTPPLWorker
    delay_flt   sippy_math.RecFilter
    _is_local    bool
    wi          chan *rtpp_req_stream
    global_config sippy_conf.Config
}

func newRtp_proxy_client_stream(owner sippy_types.RtpProxyClient, global_config sippy_conf.Config, address net.Addr, bind_address *sippy_net.HostPort) (rtp_proxy_transport, error) {
    var err error
    if address == nil {
        address, err = net.ResolveUnixAddr("unix", "/var/run/rtpproxy.sock")
        if err != nil {
            return nil, err
        }
    }
    nworkers := 4
    if owner.GetOpts().GetNWorkers() != nil {
        nworkers = *owner.GetOpts().GetNWorkers()
    }
    self := &Rtp_proxy_client_stream{
        owner       : owner,
        _address    : address,
        nworkers    : nworkers,
        workers     : make([]*_RTPPLWorker, nworkers),
        delay_flt   : sippy_math.NewRecFilter(0.95, 0.25),
        wi          : make(chan *rtpp_req_stream, 1000),
        global_config : global_config,
    }
    if strings.HasPrefix(address.Network(), "unix") {
        self._is_local = true
    } else {
        self._is_local = false
    }
    //self.wi_available = Condition()
    //self.wi = []
    for i := 0; i < self.nworkers; i++ {
        self.workers[i] = newRTPPLWorker(self)
    }
    return self, nil
}

func (self *Rtp_proxy_client_stream) is_local() bool {
    return self._is_local
}

func (self *Rtp_proxy_client_stream) address() net.Addr {
    return self._address
}

func (self *Rtp_proxy_client_stream) send_command(command string, result_callback func(string)) {
    if command[len(command)-1] != '\n' {
        command += "\n"
    }
    self.wi <- &rtpp_req_stream{ command, result_callback }
}

func (self *Rtp_proxy_client_stream) reconnect(address net.Addr, bind_addr *sippy_net.HostPort) {
    self.shutdown()
    self._address = address
    self.workers = make([]*_RTPPLWorker, self.nworkers)
    for i := 0; i < self.nworkers; i++ {
        self.workers[i] = newRTPPLWorker(self)
    }
    self.delay_flt = sippy_math.NewRecFilter(0.95, 0.25)
}

func (self *Rtp_proxy_client_stream) shutdown() {
    self.wi <- nil
    for _, rworker := range self.workers {
        <-rworker.shutdown_chan
    }
    self.workers = nil
    <-self.wi // take away the shutdown request
}

func (self *Rtp_proxy_client_stream) register_delay(rtpc_delay time.Duration) {
    self.delay_flt.Apply(rtpc_delay.Seconds())
}

func (self *Rtp_proxy_client_stream) get_rtpc_delay() float64 {
    return self.delay_flt.GetLastval()
}
/*
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
