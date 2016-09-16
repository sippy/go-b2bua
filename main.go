//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2014 Sippy Software, Inc. All rights reserved.
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
    crand "crypto/rand"
    mrand "math/rand"
    "flag"
    "os"
    "os/signal"
    "runtime"
    "strings"
    "strconv"
    "sync"
    "syscall"
    //"time"

    "sippy"
    "sippy/conf"
    "sippy/types"
    "sippy/log"
)

type callController struct {
    config          *myconfig
    logger          sippy_log.ErrorLogger
    sip_tm          sippy_types.SipTransactionManager
    uaA             sippy_types.UA
    uaO             sippy_types.UA
    lock            *sync.Mutex
}

func NewCallController(config *myconfig, sip_tm sippy_types.SipTransactionManager, logger sippy_log.ErrorLogger) *callController {
    self := &callController{
        config          : config,
        sip_tm          : sip_tm,
        logger          : logger,
        uaO             : nil,
        lock            : new(sync.Mutex),
    }
    self.uaA = sippy.NewUA(sip_tm, config, config.nh_addr, self, self.lock, nil)
    //self.uaA.SetCreditTime(5 * time.Second)
    return self
}

func (self *callController) RecvEvent(event sippy_types.CCEvent, ua sippy_types.UA) {
    if ua == self.uaA {
        if self.uaO == nil {
            if _, ok := event.(*sippy.CCEventTry); ! ok {
                // Some weird event received
                self.uaA.RecvEvent(sippy.NewCCEventDisconnect(nil, event.GetRtime(), ""))
                return
            }
            self.uaO = sippy.NewUA(self.sip_tm, self.config, self.config.nh_addr, self, self.lock, nil)
            self.uaO.SetRAddr(self.config.nh_addr)
        }
        self.uaO.RecvEvent(event)
    } else {
        self.uaA.RecvEvent(event)
    }
}

type callMap struct {
    config *myconfig
    logger          sippy_log.ErrorLogger
    sip_tm          sippy_types.SipTransactionManager
    proxy           sippy_types.StatefulProxy
}

func NewCallMap(config *myconfig, logger sippy_log.ErrorLogger) *callMap {
    return &callMap{
        logger          : logger,
        config          : config,
    }
}

func (self *callMap) OnNewDialog(req sippy_types.SipRequest, tr sippy_types.ServerTransaction) (sippy_types.UA, sippy_types.RequestReceiver, sippy_types.SipResponse) {
    if req.GetTo().GetTag() != "" {
        // Request within dialog, but no such dialog
        return nil, nil, req.GenResponse(481, "Call Leg/Transaction Does Not Exist", nil, nil)
    }
    if req.GetMethod() == "INVITE" {
        // New dialog
        cc := NewCallController(self.config, self.sip_tm, self.logger)
        return cc.uaA, cc.uaA, nil
    }
    if req.GetMethod() == "REGISTER" {
        // Registration
        return nil, self.proxy, nil
    }
    if req.GetMethod() == "NOTIFY" || req.GetMethod() == "PING" {
        // Whynot?
        return nil, nil, req.GenResponse(200, "OK", nil, nil)
    }
    return nil, nil, req.GenResponse(501, "Not Implemented", nil, nil)
}

type myconfig struct {
    sippy_conf.Config

    nh_addr *sippy_conf.HostPort
}

func main() {
    runtime.GOMAXPROCS(runtime.NumCPU())
    buf := make([]byte, 8)
    crand.Read(buf)
    var salt int64
    for _, c := range buf {
        salt = (salt << 8) | int64(c)
    }
    mrand.Seed(salt)

    var laddr, nh_addr, logfile string
    var lport int
    var foreground bool

    flag.StringVar(&laddr, "l", "", "Local addr")
    flag.IntVar(&lport, "p", -1, "Local port")
    flag.StringVar(&nh_addr, "n", "", "Next hop address")
    flag.BoolVar(&foreground, "f", false, "Run in foreground")
    flag.StringVar(&logfile, "L", "/var/log/sip.log", "Log file")
    flag.Parse()

    error_logger := sippy_log.NewErrorLogger()
    sip_logger := sippy_log.NewSipLogger("b2bua")
    config := &myconfig{
        Config : sippy_conf.NewConfig(error_logger, sip_logger),
        nh_addr      : sippy_conf.NewHostPort("192.168.0.102", "5060"), // next hop address
    }
    config.SetIPV6Enabled(false)
    if nh_addr != "" {
        var parts []string
        var addr string

        if strings.HasPrefix(nh_addr, "[") {
            parts = strings.SplitN(nh_addr, "]", 2)
            addr = parts[0] + "]"
            if len(parts) == 2 {
                parts = strings.SplitN(parts[1], ":", 2)
            }
        } else {
            parts = strings.SplitN(nh_addr, ":", 2)
            addr = parts[0]
        }
        port := "5060"
        if len(parts) == 2 {
            port = parts[1]
        }
        config.nh_addr = sippy_conf.NewHostPort(addr, port)
    }
    config.SetMyUAName("Sippy B2BUA (Simple)")
    config.SetAllowFormats([]int{ 0, 8, 18, 100, 101 })
    if laddr != "" {
        config.SetSipAddress(sippy_conf.NewMyAddress(laddr))
    } else {
        config.SetSipAddress(config.GetMyAddress())
    }
    if lport > 0 {
        config.SetSipPort(sippy_conf.NewMyPort(strconv.Itoa(lport)))
    } else {
        config.SetSipPort(config.GetMyPort())
    }
    cmap := NewCallMap(config, error_logger)
    sip_tm, err := sippy.NewSipTransactionManager(config, cmap)
    if err != nil {
        error_logger.Error(err)
        return
    }
    cmap.sip_tm = sip_tm
    cmap.proxy = sippy.NewStatefulProxy(sip_tm, config.nh_addr, config)
    go sip_tm.Run()

    signal_chan := make(chan os.Signal, 1)
    signal.Notify(signal_chan, syscall.SIGTERM, syscall.SIGINT)
    signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE, syscall.SIGUSR1, syscall.SIGUSR2)
    select {
    case <-signal_chan:
        sip_tm.Shutdown()
        break
    }
}
