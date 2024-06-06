//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2024 Sippy Software, Inc. All rights reserved.
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
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"

    "github.com/sippy/go-b2bua/sippy/cli"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

type CallMap struct {
    global_config   *myConfigParser
    ccmap           map[int64]*callController
    ccmap_lock      sync.Mutex
    gc_timeout      time.Duration
    debug_mode      bool
    safe_restart    bool
    Sip_tm          sippy_types.SipTransactionManager
    Proxy           sippy_types.StatefulProxy
    cc_id           int64
    cc_id_lock      sync.Mutex
    rtp_proxy_clients []sippy_types.RtpProxyClient
    static_route    *B2BRoute
    radius_client   *RadiusClient
    radius_auth     *RadiusAuthorisation
}

func NewCallMap(global_config *myConfigParser, rtp_proxy_clients []sippy_types.RtpProxyClient,
  static_route *B2BRoute, radius_client *RadiusClient, radius_auth *RadiusAuthorisation) *CallMap {
    self := &CallMap{
        global_config   : global_config,
        ccmap           : make(map[int64]*callController),
        gc_timeout      : time.Minute,
        debug_mode      : false,
        safe_restart    : false,
        rtp_proxy_clients: rtp_proxy_clients,
        static_route    : static_route,
        radius_client   : radius_client,
        radius_auth     : radius_auth,
    }
    go func() {
        sighup_ch := make(chan os.Signal, 1)
        signal.Notify(sighup_ch, syscall.SIGHUP)
        sigusr2_ch := make(chan os.Signal, 1)
        signal.Notify(sigusr2_ch, syscall.SIGUSR2)
        sigprof_ch := make(chan os.Signal, 1)
        signal.Notify(sigprof_ch, syscall.SIGPROF)
        sigterm_ch := make(chan os.Signal, 1)
        signal.Notify(sigterm_ch, syscall.SIGTERM)
        for {
            select {
            case <-sighup_ch:
                self.discAll(syscall.SIGHUP)
            case <-sigusr2_ch:
                self.toggleDebug()
            case <-sigprof_ch:
                self.safeRestart()
            case <-sigterm_ch:
                self.safeStop()
            }
        }
    }()
    go func() {
        for {
            time.Sleep(self.gc_timeout)
            self.GClector()
        }
    }()
    return self
}

func (self *CallMap) OnNewDialog(req sippy_types.SipRequest, sip_t sippy_types.ServerTransaction) (sippy_types.UA, sippy_types.RequestReceiver, sippy_types.SipResponse) {
    to_body, err := req.GetTo().GetBody(self.global_config)
    if err != nil {
        self.global_config.ErrorLogger().Error("CallMap::OnNewDialog: #1: " + err.Error())
        return nil, nil, req.GenResponse(500, "Internal Server Error", nil, nil)
    }
    //except Exception as exception:
        //println(datetime.now(), "can\"t parse SIP request: %s:\n" % str(exception))
        //println( "-" * 70)
        //print_exc(file = sys.stdout)
        //println( "-" * 70)
        //println(req)
        //println("-" * 70)
        //sys.stdout.flush()
        //return (nil, nil, nil)
    if to_body.GetTag() != "" {
        // Request within dialog, but no such dialog
        return nil, nil, req.GenResponse(481, "Call Leg/Transaction Does Not Exist", nil, nil)
    }
    if req.GetMethod() == "INVITE" {
        // New dialog
        var via *sippy_header.SipViaBody
        vias := req.GetVias()
        if len(vias) > 1 {
            via, err = vias[1].GetBody()
        } else {
            via, err = vias[0].GetBody()
        }
        if err != nil {
            self.global_config.ErrorLogger().Error("CallMap::OnNewDialog: #2: " + err.Error())
            return nil, nil, req.GenResponse(500, "Internal Server Error", nil, nil)
        }
        remote_ip := via.GetTAddr(self.global_config).Host
        source := req.GetSource()

        // First check if request comes from IP that
        // we want to accept our traffic from
        if ! self.global_config.checkIP(source.Host.String())  {
            return nil, nil, req.GenResponse(403, "Forbidden", nil, nil)
        }
        var challenge *sippy_header.SipWWWAuthenticate
        if self.global_config.Auth_enable {
            // Prepare challenge if no authorization header is present.
            // Depending on configuration, we might try remote ip auth
            // first and then challenge it or challenge immediately.
            if self.global_config.Digest_auth && req.GetFirstHF("authorization") == nil {
                challenge = sippy_header.NewSipWWWAuthenticateWithRealm(req.GetRURI().Host.String(), "", req.GetRtime().Realt())
            }
            // Send challenge immediately if digest is the
            // only method of authenticating
            if challenge != nil && self.global_config.Digest_auth_only {
                resp := req.GenResponse(401, "Unauthorized", nil, nil)
                resp.AppendHeader(challenge)
                return nil, nil, resp
            }
        }
        pass_headers := []sippy_header.SipHeader{}
        for _, header := range self.global_config.Pass_headers_arr {
            hfs := req.GetHFs(header)
            pass_headers = append(pass_headers, hfs...)
        }
        self.cc_id_lock.Lock()
        id := self.cc_id
        self.cc_id++
        self.cc_id_lock.Unlock()
        cguid := req.GetCGUID()
        if cguid == nil && req.GetH323ConfId() != nil {
            cguid = req.GetH323ConfId().AsCiscoGUID()
        }
        if cguid == nil {
            cguid = sippy_header.NewSipCiscoGUID()
        }
        cc := NewCallController(id, remote_ip, source, self.global_config, pass_headers, self.Sip_tm, cguid, self)
        cc.challenge = challenge
//        rval := cc.uaA.RecvRequest(req, sip_t) // this call is made by SipTransactionManager. It's necessary for for proper locking.
        self.ccmap_lock.Lock()
        self.ccmap[id] = cc
        self.ccmap_lock.Unlock()
        return cc.uaA, cc.uaA, nil
    }
    if self.Proxy != nil && (req.GetMethod() == "REGISTER" || req.GetMethod() == "SUBSCRIBE") {
        return nil, self.Proxy, nil
    }
    if (req.GetMethod() == "NOTIFY" || req.GetMethod() == "PING") {
        // Whynot?
        return nil, nil, req.GenResponse(200, "OK", nil, nil)
    }
    return nil, nil, req.GenResponse(501, "Not Implemented", nil, nil)
}

func (self CallMap) safeStop() {
    self.discAll(0)
    time.Sleep(time.Second)
    os.Exit(0)
}

func (self *CallMap) discAll(signum syscall.Signal) {
    if signum > 0 {
        println(fmt.Sprintf("Signal %d received, disconnecting all calls", signum))
    }
    alist := []*callController{}
    self.ccmap_lock.Lock()
    for _, cc := range self.ccmap {
        alist = append(alist, cc)
    }
    self.ccmap_lock.Unlock()
    for _, cc := range alist {
        cc.disconnect(nil)
    }
}

func (self *CallMap) toggleDebug() {
    if self.debug_mode {
        println("Signal received, toggling extra debug output off")
    } else {
        println("Signal received, toggling extra debug output on")
    }
    self.debug_mode = ! self.debug_mode
}

func (self *CallMap) safeRestart() {
    println("Signal received, scheduling safe restart")
    self.safe_restart = true
}

func (self *CallMap) GClector() {
    fmt.Printf("GC is invoked, %d calls in map\n", len(self.ccmap))
    if self.debug_mode {
        //println(self.global_config["_sip_tm"].tclient, self.global_config["_sip_tm"].tserver)
        for _, cc := range self.ccmap {
            println(cc.uaA.GetStateName(), cc.uaO.GetStateName())
        }
    //} else {
    //    fmt.Printf("[%d]: %d client, %d server transactions in memory\n",
    //      os.getpid(), len(self.global_config["_sip_tm"].tclient), len(self.global_config["_sip_tm"].tserver))
    }
    if self.safe_restart {
        if len(self.ccmap) == 0 {
            self.Sip_tm.Shutdown()
            //os.chdir(self.global_config["_orig_cwd"])
            cmd := exec.Command(os.Args[0], os.Args[1:]...)
            cmd.Env = os.Environ()
            err := cmd.Start()
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
            os.Exit(0)
            // Should not reach this point!
        }
        self.gc_timeout = time.Second
    }
}

func (self *CallMap) RecvCommand(clim sippy_cli.CLIManagerIface, data string) {
    args := strings.Split(strings.TrimSpace(data), " ")
    cmd := strings.ToLower(args[0])
    args = args[1:]
    switch cmd {
    case "q":
        clim.Close()
        return
    case "l":
        res := "In-memory calls:\n"
        total := 0
        self.ccmap_lock.Lock()
        defer self.ccmap_lock.Unlock()
        for _, cc := range self.ccmap {
            cc.lock.Lock()
            res += fmt.Sprintf("%s: %s (", cc.cId.CallId, cc.state.String())
            if cc.uaA != nil {
                res += fmt.Sprintf("%s %s %s %s -> ", cc.uaA.GetStateName(), cc.uaA.GetRAddr0().String(),
                  cc.uaA.GetCLD(), cc.uaA.GetCLI())
            } else {
                res += "N/A -> "
            }
            if cc.uaO != nil {
                res += fmt.Sprintf("%s %s %s %s)\n", cc.uaO.GetStateName(), cc.uaO.GetRAddr0().String(),
                  cc.uaO.GetCLI(), cc.uaO.GetCLD())
            } else {
                res += "N/A)\n"
            }
            total += 1
            cc.lock.Unlock()
        }
        clim.Send(res + fmt.Sprintf("Total: %d\n", total))
        return
/*
    case "lt":
        res = "In-memory server transactions:\n"
        for tid, t in self.global_config["_sip_tm"].tserver.iteritems() {
            res += "%s %s %s\n" % (tid, t.method, t.state)
        }
        res += "In-memory client transactions:\n"
        for tid, t in self.global_config["_sip_tm"].tclient.iteritems():
            res += "%s %s %s\n" % (tid, t.method, t.state)
        return res
    case "lt", "llt":
        if cmd == "llt":
            mindur = 60.0
        else:
            mindur = 0.0
        ctime = time()
        res = "In-memory server transactions:\n"
        for tid, t in self.global_config["_sip_tm"].tserver.iteritems():
            duration = ctime - t.rtime
            if duration < mindur:
                continue
            res += "%s %s %s %s\n" % (tid, t.method, t.state, duration)
        res += "In-memory client transactions:\n"
        for tid, t in self.global_config["_sip_tm"].tclient.iteritems():
            duration = ctime - t.rtime
            if duration < mindur:
                continue
            res += "%s %s %s %s\n" % (tid, t.method, t.state, duration)
        return res
*/
    case "d":
        if len(args) != 1 {
            clim.Send("ERROR: syntax error: d <call-id>\n")
            return
        }
        if args[0] == "*" {
            self.discAll(0)
            clim.Send("OK\n")
            return
        }
        dlist := []*callController{}
        self.ccmap_lock.Lock()
        for _, cc := range self.ccmap {
            if cc.cId.CallId != args[0] {
                continue
            }
            dlist = append(dlist, cc)
        }
        self.ccmap_lock.Unlock()
        if len(dlist) == 0 {
            clim.Send(fmt.Sprintf("ERROR: no call with id of %s has been found\n", args[0]))
            return
        }
        for _, cc := range dlist {
            cc.disconnect(nil)
        }
        clim.Send("OK\n")
        return
    case "r":
        if len(args) != 1 {
            clim.Send("ERROR: syntax error: r [<id>]\n")
            return
        }
        idx, err := strconv.ParseInt(args[0], 10, 64)
        if err != nil {
            clim.Send("ERROR: non-integer argument: " + args[0] + "\n")
            return
        }
        self.ccmap_lock.Lock()
        cc, ok := self.ccmap[idx]
        self.ccmap_lock.Unlock()
        if ! ok {
            clim.Send(fmt.Sprintf("ERROR: no call with id of %d has been found\n", idx))
            return
        }
        if cc.proxied {
            ts, _ := sippy_time.NewMonoTime()
            ts = ts.Add(-60 * time.Second)
            if cc.state == CCStateConnected {
                cc.disconnect(ts)
            } else if cc.state == CCStateARComplete {
                cc.uaO.Disconnect(ts, "")
            }
        }
        clim.Send("OK\n")
        return
    default:
        clim.Send("ERROR: unknown command\n")
    }
}

func (self *CallMap) DropCC(cc_id int64) {
    self.ccmap_lock.Lock()
    delete(self.ccmap, cc_id)
    self.ccmap_lock.Unlock()
}
