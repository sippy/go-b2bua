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
// #include <sys/socket.h>
//
// #ifdef SO_REUSEPORT
// #define SO_REUSEPORT_EXISTS 1
// #else
// #define SO_REUSEPORT_EXISTS 0
// #define SO_REUSEPORT 0 /* just a placeholder to keep the go code compilable */
// #endif
//
import "C"
import (
    "net"
    "os"
    "runtime"
    "strconv"
    "syscall"
    "time"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/log"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/utils"
)

type write_req struct {
    address     net.Addr
    data        []byte
    on_complete func()
}

type resolv_req struct {
    hostport    *sippy_net.HostPort
    data        []byte
}

type asyncResolver struct {
    sem         chan int
    logger      sippy_log.ErrorLogger
}

func NewAsyncResolver(userv *UdpServer, logger sippy_log.ErrorLogger) *asyncResolver {
    self := &asyncResolver{
        sem     : make(chan int, 2),
        logger  : logger,
    }
    go self.run(userv)
    return self
}

func (self *asyncResolver) run(userv *UdpServer) {
    var wi *resolv_req
LOOP:
    for {
        wi = <-userv.wi_resolv
        if wi == nil {
            // Shutdown request, relay it further
            userv.wi_resolv <- nil
            break LOOP
        }
        start, _ := sippy_time.NewMonoTime()
        addr, err := net.ResolveUDPAddr("udp", wi.hostport.String())
        delay, _ := start.OffsetFromNow()
        if err != nil {
            self.logger.Errorf("Udp_server: Cannot resolve '%s', dropping outgoing SIP message. Delay %s", wi.hostport, delay.String())
            continue
        }
        if delay > time.Duration(.5 * float64(time.Second)) {
            self.logger.Error("Udp_server: DNS resolve time for '%s' is too big: %s", wi.hostport, delay.String())
        }
        userv._send_to(wi.data, addr, nil)
    }
    self.sem <- 1
}

type asyncSender struct {
    sem     chan int
}

func NewAsyncSender(userv *UdpServer, n int) *asyncSender {
    self := &asyncSender{
        sem     : make(chan int, 2),
    }
    go self.run(userv)
    return self
}

func (self *asyncSender) run(userv *UdpServer) {
    var wi *write_req
LOOP:
    for {
        wi = <-userv.wi
        if wi == nil { // shutdown req
            userv.wi <- nil
            break LOOP
        }
SEND_LOOP:
        for i := 0; i < 20; i++ {
            if _, err := userv.skt.WriteTo(wi.data, wi.address); err == nil {
                if wi.on_complete != nil {
                    wi.on_complete()
                }
                break SEND_LOOP
            }
            time.Sleep(10 * time.Millisecond)
        }
    }
    self.sem <- 1
}

type asyncReceiver struct {
    sem             chan int
    logger          sippy_log.ErrorLogger
}

func NewAsyncReciever(userv *UdpServer, logger sippy_log.ErrorLogger) *asyncReceiver {
    self := &asyncReceiver{
        sem     : make(chan int, 2),
        logger  : logger,
    }
    go self.run(userv)
    return self
}

func (self *asyncReceiver) run(userv *UdpServer) {
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
        msg := make([]byte, 0, n)
        msg = append(msg, buf[:n]...)
        sippy_utils.SafeCall(func() { userv.handle_read(msg, address, rtime) }, nil, self.logger)
    }
    self.sem <- 1
}

type udpServerOpts struct {
    laddress        *sippy_net.HostPort
    data_callback   sippy_net.DataPacketReceiver
    nworkers        int
}

func NewUdpServerOpts(laddress *sippy_net.HostPort, data_callback sippy_net.DataPacketReceiver) *udpServerOpts {
    self := &udpServerOpts{
        laddress        : laddress,
        data_callback   : data_callback,
        nworkers        : runtime.NumCPU() * 2,
    }
    return self
}

type UdpServer struct {
    uopts           udpServerOpts
    skt             net.PacketConn
    wi              chan *write_req
    wi_resolv       chan *resolv_req
    asenders        []*asyncSender
    areceivers      []*asyncReceiver
    aresolvers      []*asyncResolver
    packets_recvd int
    packets_sent int
    packets_queued int
}

func zoneToUint32(zone string) uint32 {
    if zone == "" {
        return 0
    }
    if ifi, err := net.InterfaceByName(zone); err == nil {
        return uint32(ifi.Index)
    }
    n, err := strconv.Atoi(zone)
    if err != nil {
        return 0
    }
    return uint32(n)
}

func NewUdpServer(config sippy_conf.Config, uopts *udpServerOpts) (*UdpServer, error) {
    var laddress *net.UDPAddr
    var err error
    var ip4 net.IP

    proto := syscall.AF_INET
    if uopts.laddress != nil {
        laddress, err = net.ResolveUDPAddr("udp", uopts.laddress.String())
        if err != nil {
            return nil, err
        }
        if sippy_net.IsIP4(laddress.IP) {
            ip4 = laddress.IP.To4()
        } else {
            proto = syscall.AF_INET6
        }
    }
    s, err := syscall.Socket(proto, syscall.SOCK_DGRAM, 0)
    if err != nil { return nil, err }
    if laddress != nil {
        if err = syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
            syscall.Close(s)
            return nil, err
        }
        if C.SO_REUSEPORT_EXISTS == 1 {
            if err = syscall.SetsockoptInt(s, syscall.SOL_SOCKET, C.SO_REUSEPORT, 1); err != nil {
                syscall.Close(s)
                return nil, err
            }
        }
        var sockaddr syscall.Sockaddr
        if ip4 != nil {
            sockaddr = &syscall.SockaddrInet4{
                Port : laddress.Port,
                Addr : [4]byte{ ip4[0], ip4[1], ip4[2], ip4[3] },
            }
        } else {
            sa6 := &syscall.SockaddrInet6{
                Port : laddress.Port,
                ZoneId : zoneToUint32(laddress.Zone),
            }
            for i := 0; i < 16; i++ {
                sa6.Addr[i] = laddress.IP[i]
            }
            sockaddr = sa6
        }
        if err := syscall.Bind(s, sockaddr); err != nil {
            syscall.Close(s)
            return nil, err
        }
    }
    f := os.NewFile(uintptr(s), "")
    skt, err := net.FilePacketConn(f)
    f.Close()
    if err != nil {
        return nil, err
    }
    self := &UdpServer{
        uopts       : *uopts,
        skt         : skt,
        wi          : make(chan *write_req, 1000),
        wi_resolv   : make(chan *resolv_req, 1000),
        asenders    : make([]*asyncSender, 0, uopts.nworkers),
        areceivers  : make([]*asyncReceiver, 0, uopts.nworkers),
        aresolvers  : make([]*asyncResolver, 0, uopts.nworkers),
    }
    for n := 0; n < uopts.nworkers; n++ {
        self.asenders = append(self.asenders, NewAsyncSender(self, n))
        self.areceivers = append(self.areceivers, NewAsyncReciever(self, config.ErrorLogger()))
    }
    for n:= 0; n < uopts.nworkers; n++ {
        self.aresolvers = append(self.aresolvers, NewAsyncResolver(self, config.ErrorLogger()))
    }
    return self, nil
}

func (self *UdpServer) SendTo(data []byte, hostport *sippy_net.HostPort) {
    self.SendToWithCb(data, hostport, nil)
}

func (self *UdpServer) SendToWithCb(data []byte, hostport *sippy_net.HostPort, on_complete func()) {
    ip := hostport.ParseIP()
    if ip == nil {
        self.wi_resolv <- &resolv_req{ data : data, hostport : hostport }
        return
    }
    address, err := net.ResolveUDPAddr("udp", hostport.String()) // in fact no resolving is done here
    if err != nil {
        return // not reached
    }
    self._send_to(data, address, on_complete)
}

func (self *UdpServer) _send_to(data []byte, address net.Addr, on_complete func()) {
    self.wi <- &write_req{
        data        : data,
        address     : address,
        on_complete : on_complete,
    }
}

func (self *UdpServer) handle_read(data []byte, address net.Addr, rtime *sippy_time.MonoTime) {
    if len(data) > 0 {
        self.packets_recvd++
        host, port, _ := net.SplitHostPort(address.String())
        self.uopts.data_callback(data, sippy_net.NewHostPort(host, port), self, rtime)
    }
}

func (self *UdpServer) Shutdown() {
    // shutdown the senders and resolvers first
    self.wi <- nil
    self.wi_resolv <- nil
    for _, worker := range self.asenders { <-worker.sem }
    for _, worker := range self.aresolvers { <-worker.sem }
    self.skt.Close()

    for _, worker := range self.areceivers { <-worker.sem }
    self.asenders = make([]*asyncSender, 0)
    self.areceivers = make([]*asyncReceiver, 0)
    self.aresolvers = make([]*asyncResolver, 0)
}

func (self *UdpServer) GetLAddress() *sippy_net.HostPort {
    return self.uopts.laddress
}
