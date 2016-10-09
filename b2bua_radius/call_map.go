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
    "os"
    "os/signal"
    "syscall"
)

type callMap struct {
    global_config   *myConfigParser
    ccmap           map[int64]*callController
}

/*
class CallMap(object):
    ccmap = nil
    el = nil
    debug_mode = false
    safe_restart = false
    global_config = nil
    proxy = nil
    //rc1 = nil
    //rc2 = nil
*/

func NewCallMap(global_config *myConfigParser) *callMap {
    self := &callMap{
        global_config   : global_config,
        ccmap           : make(map[int64]*callController),
    }
    go func() {
        sighup_ch := make(chan os.Signal, 1)
        signal.Notify(sighup_ch, syscall.SIGHUP)
        sigusr2_ch := make(chan os.Signal, 1)
        signal.Notify(sigusr2_ch, syscall.SIGUSR2)
        sigprof_ch := make(chan os.Signal, 1)
        signal.Notify(sigprof_ch, syscall.SIGPROF)
        for {
            select {
            case <-sighup_ch:
                self.discAll(syscall.SIGHUP)
            case <-sigusr2_ch:
                self.toggleDebug()
            case <-sigprof_ch:
                self.safeRestart()
            }
        }
    }()
    self.el = Timeout(self.GClector, 60, -1)
}
/*
    def recvRequest(self, req, sip_t):
        try:
            to_tag = req.getHFBody("to").getTag()
        except Exception as exception:
            println(datetime.now(), "can\"t parse SIP request: %s:\n" % str(exception))
            println( "-" * 70)
            print_exc(file = sys.stdout)
            println( "-" * 70)
            println(req)
            println("-" * 70)
            sys.stdout.flush()
            return (nil, nil, nil)
        if to_tag != nil:
            // Request within dialog, but no such dialog
            return (req.genResponse(481, "Call Leg/Transaction Does Not Exist"), nil, nil)
        if req.getMethod() == "INVITE":
            // New dialog
            if req.countHFs("via") > 1:
                via = req.getHFBody("via", 1)
            else:
                via = req.getHFBody("via", 0)
            remote_ip = via.getTAddr()[0]
            source = req.getSource()

            // First check if request comes from IP that
            // we want to accept our traffic from
            if self.global_config.has_key("_accept_ips") && \
              ! source[0] in self.global_config["_accept_ips"]:
                resp = req.genResponse(403, "Forbidden")
                return (resp, nil, nil)

            challenge = nil
            if self.global_config["auth_enable"]:
                // Prepare challenge if no authorization header is present.
                // Depending on configuration, we might try remote ip auth
                // first and then challenge it or challenge immediately.
                if self.global_config["digest_auth"] && \
                  req.countHFs("authorization") == 0:
                    challenge = SipHeader(name = "www-authenticate")
                    challenge.getBody().realm = req.getRURI().host
                // Send challenge immediately if digest is the
                // only method of authenticating
                if challenge != nil && self.global_config.getdefault("digest_auth_only", false):
                    resp = req.genResponse(401, "Unauthorized")
                    resp.appendHeader(challenge)
                    return (resp, nil, nil)

            pass_headers = []
            for header in self.global_config["_pass_headers"]:
                hfs = req.getHFs(header)
                if len(hfs) > 0:
                    pass_headers.extend(hfs)
            cc = CallController(remote_ip, source, self.global_config, pass_headers)
            cc.challenge = challenge
            rval = cc.uaA.recvRequest(req, sip_t)
            self.ccmap.append(cc)
            return rval
        if self.proxy != nil && req.getMethod() in ("REGISTER", "SUBSCRIBE"):
            return self.proxy.recvRequest(req)
        if req.getMethod() in ("NOTIFY", "PING"):
            // Whynot?
            return (req.genResponse(200, "OK"), nil, nil)
        return (req.genResponse(501, "Not Implemented"), nil, nil)
*/
func (self *callMap) discAll(signum syscall.Signal) {
    if signum > 0 {
        println("Signal %d received, disconnecting all calls" % signum)
    }
    for _, cc := range self.ccmap {
        cc.disconnect()
    }
}

func (self *callMap) toggleDebug() {
    if self.debug_mode {
        println("Signal received, toggling extra debug output off")
    } else {
        println("Signal received, toggling extra debug output on")
    }
    self.debug_mode = ! self.debug_mode
}

func (self *callMap) safeRestart() {
    println("Signal received, scheduling safe restart")
    self.safe_restart = true
}

/*
    def GClector(self):
        println("GC is invoked, %d calls in map" % len(self.ccmap))
        if self.debug_mode:
            println(self.global_config["_sip_tm"].tclient, self.global_config["_sip_tm"].tserver)
            for cc in tuple(self.ccmap):
                try:
                    println(cc.uaA.state, cc.uaO.state)
                except AttributeError:
                    println(nil)
        else:
            println("[%d]: %d client, %d server transactions in memory" % \
              (os.getpid(), len(self.global_config["_sip_tm"].tclient), len(self.global_config["_sip_tm"].tserver)))
        if self.safe_restart:
            if len(self.ccmap) == 0:
                self.global_config["_sip_tm"].userv.close()
                os.chdir(self.global_config["_orig_cwd"])
                argv = [sys.executable,]
                argv.extend(self.global_config["_orig_argv"])
                os.execv(sys.executable, argv)
                // Should not reach this point!
            self.el.ival = 1
        //print gc.collect()
        if len(gc.garbage) > 0:
            println(gc.garbage)

    def recvCommand(self, clim, cmd):
        args = cmd.split()
        cmd = args.pop(0).lower()
        if cmd == "q":
            clim.close()
            return false
        if cmd == "l":
            res = "In-memory calls:\n"
            total = 0
            for cc in self.ccmap:
                res += "%s: %s (" % (cc.cId, cc.state.sname)
                if cc.uaA != nil:
                    res += "%s %s:%d %s %s -> " % (cc.uaA.state, cc.uaA.getRAddr0()[0], \
                      cc.uaA.getRAddr0()[1], cc.uaA.getCLD(), cc.uaA.getCLI())
                else:
                    res += "N/A -> "
                if cc.uaO != nil:
                    res += "%s %s:%d %s %s)\n" % (cc.uaO.state, cc.uaO.getRAddr0()[0], \
                      cc.uaO.getRAddr0()[1], cc.uaO.getCLI(), cc.uaO.getCLD())
                else:
                    res += "N/A)\n"
                total += 1
            res += "Total: %d\n" % total
            clim.send(res)
            return false
        if cmd == "lt":
            res = "In-memory server transactions:\n"
            for tid, t in self.global_config["_sip_tm"].tserver.iteritems():
                res += "%s %s %s\n" % (tid, t.method, t.state)
            res += "In-memory client transactions:\n"
            for tid, t in self.global_config["_sip_tm"].tclient.iteritems():
                res += "%s %s %s\n" % (tid, t.method, t.state)
            clim.send(res)
            return false
        if cmd in ("lt", "llt"):
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
            clim.send(res)
            return false
        if cmd == "d":
            if len(args) != 1:
                clim.send("ERROR: syntax error: d <call-id>\n")
                return false
            if args[0] == "*":
                self.discAll()
                clim.send("OK\n")
                return false
            dlist = [x for x in self.ccmap if str(x.cId) == args[0]]
            if len(dlist) == 0:
                clim.send("ERROR: no call with id of %s has been found\n" % args[0])
                return false
            for cc in dlist:
                cc.disconnect()
            clim.send("OK\n")
            return false
        if cmd == "r":
            if len(args) != 1:
                clim.send("ERROR: syntax error: r [<id>]\n")
                return false
            idx = int(args[0])
            dlist = [x for x in self.ccmap if x.id == idx]
            if len(dlist) == 0:
                clim.send("ERROR: no call with id of %d has been found\n" % idx)
                return false
            for cc in dlist:
                if ! cc.proxied:
                    continue
                if cc.state == CCStateConnected:
                    cc.disconnect(time() - 60)
                    continue
                if cc.state == CCStateARComplete:
                    cc.uaO.disconnect(time() - 60)
                    continue
            clim.send("OK\n")
            return false
        clim.send("ERROR: unknown command\n")
        return false
*/
