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
package sippy

import (
    "fmt"
    "net"
    "sync"
    "time"

    "sippy/conf"
    "sippy/container"
    "sippy/log"
    "sippy/time"
)

type UdpPacketReceiver func(data []byte, addr *sippy_conf.HostPort, server *udpServer, rtime *sippy_time.MonoTime)

type write_req struct {
    address     net.Addr
    data        []byte
}

type resolv_req struct {
    hostport    string
    data        []byte
}

type shutdown_req struct {
}

type asyncResolver struct {
    sem         chan int
    logger      sippy_log.ErrorLogger
}

func NewAsyncResolver(userv *udpServer, logger sippy_log.ErrorLogger) *asyncResolver {
    self := &asyncResolver{
        sem     : make(chan int),
        logger  : logger,
    }
    go self.run(userv)
    return self
}

func (self *asyncResolver) run(userv *udpServer) {
    var wi *resolv_req
LOOP:
    for {
        userv.wi_resolv_cv.L.Lock()
        for userv.wi_resolv.IsEmpty() {
            userv.wi_resolv_cv.Wait()
        }
        wi = nil
        switch t := userv.wi_resolv.Get().Value.(type) {
        case *shutdown_req:
            // Shutdown request, relay it further
            userv.wi_resolv.Put(t)
            userv.wi_resolv_cv.Signal()
            userv.wi_resolv_cv.L.Unlock()
            break LOOP
        case *resolv_req:
            wi = t
        }
        if wi == nil { continue }
        start, _ := sippy_time.NewMonoTime()
        addr, err := net.ResolveUDPAddr("udp", wi.hostport)
        delay, _ := start.OffsetFromNow()
        if err != nil {
            self.logger.Error(fmt.Sprintf("Udp_server: Cannot resolve '%s', dropping outgoing SIP message. Delay %s", wi.hostport, delay.String()))
            continue
        }
        if delay > time.Duration(.5 * float64(time.Second)) {
            self.logger.Error("Udp_server: DNS resolve time for '%s' is too big: %s", wi.hostport, delay.String())
        }
        userv._send_to(wi.data, addr)
    }
}

type asyncSender struct {
    sem     chan int
}

func NewAsyncSender(userv *udpServer) *asyncSender {
    self := &asyncSender{
        sem     : make(chan int),
    }
    go self.run(userv)
    return self
}

func (self *asyncSender) run(userv *udpServer) {
    var wi *write_req
LOOP:
    for {
        wi = nil
        userv.wi_cv.L.Lock()
        for userv.wi.IsEmpty() {
            userv.wi_cv.Wait()
        }
        switch t := userv.wi.Get().Value.(type) {
        case *shutdown_req:
            userv.wi.Put(t)
            userv.wi_cv.Signal()
            userv.wi_cv.L.Unlock()
            break LOOP
        case *write_req:
            wi = t
        }
        userv.wi_cv.L.Unlock()
SEND_LOOP:
        for wi != nil {
            for i := 0; i < 20; i++ {
                if _, err := userv.skt.WriteTo(wi.data, wi.address); err == nil {
                    break SEND_LOOP
                }
            }
            time.Sleep(time.Duration(0.01 * float64(time.Second)))
        }
    }
    self.sem <- 1
}

type asyncReceiver struct {
    sem             chan int
    logger          sippy_log.ErrorLogger
}

func NewAsyncReciever(userv *udpServer, logger sippy_log.ErrorLogger) *asyncReceiver {
    self := &asyncReceiver{
        sem     : make(chan int),
        logger  : logger,
    }
    go self.run(userv)
    return self
}

func (self *asyncReceiver) run(userv *udpServer) {
    buf := make([]byte, 8192)
    for {
        n, address, err := userv.skt.ReadFrom(buf)
        if err != nil {
            break
        }
        rtime, err := sippy_time.NewMonoTime()
        if err != nil {
            self.logger.Error("Cannot create MonoTime object")
            continue
        }
        userv.handle_read(buf[:n], address, rtime)
    }
    self.sem <- 1
}

type udpServerOpts struct {
    laddress        *sippy_conf.HostPort
    data_callback   UdpPacketReceiver
    shut_down       bool
    nworkers        int
}

func NewUdpServerOpts(laddress *sippy_conf.HostPort, data_callback UdpPacketReceiver) *udpServerOpts {
    self := &udpServerOpts{
        laddress        : laddress,
        data_callback   : data_callback,
        nworkers        : 10,
        shut_down       : false,
    }
    return self
}

type udpServer struct {
    uopts           udpServerOpts
    skt             *net.UDPConn
    wi              sippy_container.Fifo
    wi_cv           *sync.Cond
    wi_resolv       sippy_container.Fifo
    wi_resolv_cv    *sync.Cond
    asenders        []*asyncSender
    areceivers      []*asyncReceiver
    aresolvers      []*asyncResolver
    packets_recvd int
    packets_sent int
    packets_queued int
}

func NewUdpServer(config sippy_conf.Config, uopts *udpServerOpts) (*udpServer, error) {
    var laddress *net.UDPAddr
    var err error

    if uopts.laddress != nil {
        laddress, err = net.ResolveUDPAddr("udp", uopts.laddress.String())
    } else {
        laddress, err = net.ResolveUDPAddr("udp", ":0")
    }
    if err != nil { return nil, err }
    skt, err := net.ListenUDP("udp", laddress)
    if err != nil { return nil, err }
    self := &udpServer{
        uopts       : *uopts,
        skt         : skt,
        wi          : sippy_container.NewFifo(),
        wi_cv       : sync.NewCond(new(sync.Mutex)),
        wi_resolv   : sippy_container.NewFifo(),
        wi_resolv_cv: sync.NewCond(new(sync.Mutex)),
        asenders    : make([]*asyncSender, 0, uopts.nworkers),
        areceivers  : make([]*asyncReceiver, 0, uopts.nworkers),
        aresolvers  : make([]*asyncResolver, 0, uopts.nworkers),
    }
    for n := 0; n < uopts.nworkers; n++ {
        self.asenders = append(self.asenders, NewAsyncSender(self))
        self.areceivers = append(self.areceivers, NewAsyncReciever(self, config.ErrorLogger()))
    }
    for n:= 0; n < uopts.nworkers; n++ {
        self.aresolvers = append(self.aresolvers, NewAsyncResolver(self, config.ErrorLogger()))
    }
    return self, nil
}

func (self *udpServer) SendTo(data []byte, host, port string) {
    hostport := net.JoinHostPort(host, port)
    ip := net.ParseIP(host)
    if ip == nil {
        self.wi_resolv_cv.L.Lock()
        self.wi_resolv.Put(&resolv_req{ data : data, hostport : hostport })
        self.wi_resolv_cv.Signal()
        self.wi_resolv_cv.L.Unlock()
        return
    }
    address, err := net.ResolveUDPAddr("udp", hostport) // in fact no resolving is done here
    if err != nil {
        return // not reached
    }
    self._send_to(data, address)
}

func (self *udpServer) _send_to(data []byte, address net.Addr) {
    self.wi_cv.L.Lock()
    self.wi.Put(&write_req{ data : data, address : address })
    self.wi_cv.Signal()
    self.wi_cv.L.Unlock()
}

func (self *udpServer) handle_read(data []byte, address net.Addr, rtime *sippy_time.MonoTime) {
    if len(data) > 0 {
        self.packets_recvd++
        host, port, _ := net.SplitHostPort(address.String())
        self.uopts.data_callback(data, sippy_conf.NewHostPort(host, port), self, rtime)
    }
}

func (self *udpServer) Shutdown() {
    self.skt.Close()

    self.wi_cv.L.Lock()
    self.wi.Put(&shutdown_req{})
    self.wi_cv.Signal()
    self.wi_cv.L.Unlock()

    self.wi_resolv_cv.L.Lock()
    self.wi_resolv.Put(&shutdown_req{})
    self.wi_resolv_cv.Signal()
    self.wi_resolv_cv.L.Unlock()

    self.uopts.shut_down = true // self.uopts.data_callback = None
    for _, worker := range self.asenders { <-worker.sem }
    for _, worker := range self.areceivers { <-worker.sem }
    for _, worker := range self.aresolvers { <-worker.sem }
    self.asenders = make([]*asyncSender, 0)
    self.areceivers = make([]*asyncReceiver, 0)
    self.aresolvers = make([]*asyncResolver, 0)
}

func (self *udpServer) GetLaddress() *sippy_conf.HostPort {
    return self.uopts.laddress
}
