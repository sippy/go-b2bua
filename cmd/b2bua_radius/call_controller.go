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
    "crypto/md5"
    "fmt"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/sippy/go-b2bua/sippy"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

type callController struct {
    id              int64
    username        string
    uaA             sippy_types.UA
    uaO             sippy_types.UA
    global_config   *myConfigParser
    state           CCState
    remote_ip       *sippy_net.MyAddress
    source          *sippy_net.HostPort
    routes          []*B2BRoute
    pass_headers    []sippy_header.SipHeader
    lock            *sync.Mutex // this must be a reference to prevent memory leak
    cId             *sippy_header.SipCallId
    cGUID           *sippy_header.SipCiscoGUID
    cli             string
    cld             string
    caller_name     string
    challenge       *sippy_header.SipWWWAuthenticate
    rtp_proxy_session *sippy.Rtp_proxy_session
    eTry            *sippy.CCEventTry
    huntstop_scodes []int
    acctA           Accounting
    acctO           *RadiusAccounting
    sip_tm          sippy_types.SipTransactionManager
    proxied         bool
    sdp_session     *sippy.SdpSession
    cmap            *CallMap
    auth_proc       Cancellable
}

func NewCallController(id int64, remote_ip *sippy_net.MyAddress, source *sippy_net.HostPort, global_config *myConfigParser,
  pass_headers []sippy_header.SipHeader, sip_tm sippy_types.SipTransactionManager, cguid *sippy_header.SipCiscoGUID,
  cmap *CallMap) *callController {
    self := &callController{
        id              : id,
        global_config   : global_config,
        state           : CCStateIdle,
        remote_ip       : remote_ip,
        source          : source,
        routes          : make([]*B2BRoute, 0),
        pass_headers    : pass_headers,
        lock            : new(sync.Mutex),
        huntstop_scodes : make([]int, 0),
        proxied         : false,
        sip_tm          : sip_tm,
        sdp_session     : sippy.NewSdpSession(),
        cGUID           : cguid,
        cmap            : cmap,
    }
    self.uaA = sippy.NewUA(sip_tm, global_config, nil, self, self.lock, nil)
    self.uaA.SetKaInterval(self.global_config.Keepalive_ans_dur)
    self.uaA.SetLocalUA(sippy_header.NewSipUserAgent(self.global_config.GetMyUAName()))
    self.uaA.SetConnCb(self.aConn)
    self.uaA.SetDiscCb(self.aDisc)
    self.uaA.SetFailCb(self.aFail)
    self.uaA.SetDeadCb(self.aDead)
    return self
}

func (self *callController) RecvEvent(event sippy_types.CCEvent, ua sippy_types.UA) {
    if ua == self.uaA {
        if self.state == CCStateIdle {
            ev_try, ok := event.(*sippy.CCEventTry)
            if ! ok {
                // Some weird event received
                self.uaA.RecvEvent(sippy.NewCCEventDisconnect(nil, event.GetRtime(), ""))
                return
            }
            self.cId = ev_try.GetSipCallId()
            self.cli = ev_try.GetCLI()
            self.cld = ev_try.GetCLD()
            self.caller_name = ev_try.GetCallerName()
            if self.cld == "" {
                self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (1)", event.GetRtime(), ""))
                self.state = CCStateDead
                return
            }
            body := ev_try.GetBody()
            if body != nil && len(self.global_config.Allowed_pts) > 0 {
                sdp_body, err := body.GetSdp()
                if err != nil {
                    self.uaA.RecvEvent(sippy.NewCCEventFail(400, "Malformed SDP Body", event.GetRtime(), ""))
                    self.state = CCStateDead
                    return
                }
                sections := sdp_body.GetSections()
                if len(sections) > 0 {
                    mbody := sections[0].GetMHeader()
                    if strings.ToLower(mbody.GetTransport()) == "rtp/avp" {
                        formats := []string{}
                        old_len := len(mbody.GetFormats())
                        for _, pt := range mbody.GetFormats() {
                            if _, ok := self.global_config.Allowed_pts_map[pt]; ok {
                                formats = append(formats, pt)
                            }
                        }
                        if len(formats) == 0 {
                            self.uaA.RecvEvent(sippy.NewCCEventFail(488, "Not Acceptable Here", event.GetRtime(), ""))
                            self.state = CCStateDead
                            return
                        }
                        if old_len > len(formats) {
                            sections[0].SetFormats(formats)
                        }
                    }
                }
            }
            if strings.HasPrefix(self.cld, "nat-") {
                self.cld = self.cld[4:]
                if ev_try.GetBody() != nil {
                    ev_try.GetBody().AppendAHeader("nated:yes")
                }
                event, _ = sippy.NewCCEventTry(self.cId, self.cli, self.cld, ev_try.GetBody(), ev_try.GetSipAuthorizationHF(), self.caller_name, nil, "")
            }
            if self.global_config.Static_tr_in != "" {
                var err error
                self.cld, err = re_replace(self.global_config.Static_tr_in, self.cld)
                if err != nil {
                    self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (5)", event.GetRtime(), ""))
                    self.state = CCStateDead
                    return
                }
                event, _ = sippy.NewCCEventTry(self.cId, self.cli, self.cld, ev_try.GetBody(), ev_try.GetSipAuthorizationHF(), self.caller_name, nil, "")
            }
            if len(self.cmap.rtp_proxy_clients) > 0 {
                var err error
                self.rtp_proxy_session, err = sippy.NewRtp_proxy_session(self.global_config, self.cmap.rtp_proxy_clients, self.cId.CallId, "", "", self.global_config.B2bua_socket, /*notify_tag*/ fmt.Sprintf("r%%20%d", self.id), self.lock)
                if err != nil {
                    self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (4)", event.GetRtime(), ""))
                    self.state = CCStateDead
                    return
                }
                self.rtp_proxy_session.SetCalleeRaddress(sippy_net.NewHostPort(self.remote_ip.String(), "5060"))
                self.rtp_proxy_session.SetInsertNortpp(true)
            }
            self.eTry = ev_try
            self.state = CCStateWaitRoute
            if ! self.global_config.Auth_enable {
                self.username = self.remote_ip.String()
                self.rDone_nolock(NewRadiusResult())
                return
            }
            var auth *sippy_header.SipAuthorizationBody
            var err error
            auth_hf := ev_try.GetSipAuthorizationHF()
            if auth_hf != nil {
                auth, err = auth_hf.GetBody()
                if err != nil {
                    self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (6)", event.GetRtime(), ""))
                    self.state = CCStateDead
                    return
                }
            }
            if auth == nil || auth.GetUsername() == "" {
                self.username = self.remote_ip.String()
                self.auth_proc = self.cmap.radius_auth.Do_auth(self.remote_ip.String(), self.cli, self.cld, self.cGUID,
                  self.cId, self.remote_ip, self.rDone, "", "", "", "")
            } else {
                self.username = auth.GetUsername()
                self.auth_proc = self.cmap.radius_auth.Do_auth(auth.GetUsername(), self.cli, self.cld, self.cGUID,
                  self.cId, self.remote_ip, self.rDone, auth.GetRealm(), auth.GetNonce(), auth.GetUri(), auth.GetResponse())
            }
            return
        }
        if (self.state != CCStateARComplete && self.state != CCStateConnected && self.state != CCStateDisconnecting) || self.uaO == nil {
            return
        }
        self.uaO.RecvEvent(event)
    } else {
        ev_fail, is_ev_fail := event.(*sippy.CCEventFail)
        _, is_ev_disconnect := event.(*sippy.CCEventFail)
        if (is_ev_fail || is_ev_disconnect) && self.state == CCStateARComplete &&
          (self.uaA.GetState() == sippy_types.UAS_STATE_TRYING ||
          self.uaA.GetState() == sippy_types.UAS_STATE_RINGING) && len(self.routes) > 0 {
            huntstop := false
            if is_ev_fail {
                for _, c := range self.huntstop_scodes {
                    if c == ev_fail.GetScode() {
                        huntstop = true
                        break
                    }
                }
            }
            if ! huntstop {
                route := self.routes[0]
                self.routes = self.routes[1:]
                self.placeOriginate(route)
                return
            }
        }
        self.sdp_session.FixupVersion(event.GetBody())
        self.uaA.RecvEvent(event)
    }
}

func (self *callController) rDone(results *RadiusResult) {
    self.lock.Lock()
    defer self.lock.Unlock()
    self.rDone_nolock(results)
}

func (self *callController) rDone_nolock(results *RadiusResult) {
    // Check that we got necessary result from Radius
    if results == nil || results.Rcode != 0 {
        if self.uaA.GetState() == sippy_types.UAS_STATE_TRYING {
            var event sippy_types.CCEvent
            if self.challenge != nil {
                event = sippy.NewCCEventFail(401, "Unauthorized", nil, "")
                event.AppendExtraHeader(self.challenge)
            } else {
                event = sippy.NewCCEventFail(403, "Auth Failed", nil, "")
            }
            self.uaA.RecvEvent(event)
            self.state = CCStateDead
        }
        return
    }
    if self.global_config.Acct_enable {
        acctA := NewRadiusAccounting(self.global_config, "answer", self.cmap.radius_client)
        acctA.SetParams(self.username, self.cli, self.cld, self.cGUID.StringBody(), self.cId.StringBody(), self.remote_ip.String(), "")
        self.acctA = acctA
    } else {
        self.acctA = NewFakeAccounting()
    }
    // Check that uaA is still in a valid state, send acct stop
    if self.uaA.GetState() != sippy_types.UAS_STATE_TRYING {
        rtime, _ := sippy_time.NewMonoTime()
        self.acctA.Disc(self.uaA, rtime, "caller", 0)
        return
    }
    cli := ""
    caller_name := ""
    credit_time := time.Duration(0)
    credit_time_found := false
    for _, avp := range results.Avps {
        if avp.name == "h323-ivr-in" {
            if cli == "" && strings.HasPrefix(avp.value, "CLI:") {
                cli = avp.value[4:]
            }
            if caller_name == "" && strings.HasPrefix(avp.value, "CNAM:") {
                caller_name = avp.value[5:]
            }
            if ! credit_time_found {
                credit_time_found = true
                val, err := strconv.Atoi(avp.value)
                if err == nil {
                    credit_time = time.Duration(val) * time.Second
                }
            }
        }
    }
    routing := []*B2BRoute{}

    if self.cmap.static_route == nil {
        for _, avp := range results.Avps {
            if avp.name == "h323-ivr-in" && strings.HasPrefix(avp.value, "Routing:") {
                b2br, err := NewB2BRoute(avp.value[8:], self.global_config)
                if err == nil {
                    routing = append(routing, b2br)
                }
            }
        }
        if len(routing) == 0 {
            self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (2)", nil, ""))
            self.state = CCStateDead
            return
        }
    } else {
        routing = []*B2BRoute{ self.cmap.static_route.getCopy() }
    }
    rnum := 0
    for _, oroute := range routing {
        rnum += 1
        oroute.customize(rnum, self.cld, self.cli, 0, self.pass_headers, 0)
        oroute.customize(rnum, self.cld, self.cli, credit_time, self.pass_headers, time.Duration(self.global_config.Max_credit_time) * time.Second)
        if oroute.credit_time == 0 || oroute.expires == 0 {
            continue
        }
        self.routes = append(self.routes, oroute)
        //println "Got route:", oroute.hostport, oroute.cld
    }
    if len(self.routes) == 0 {
        self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (3)", nil, ""))
        self.state = CCStateDead
        return
    }
    self.state = CCStateARComplete
    route := self.routes[0]
    self.routes = self.routes[1:]
    self.placeOriginate(route)
}

func (self *callController) placeOriginate(oroute *B2BRoute) {
    //cId, cGUID, cli, cld, body, auth, caller_name = self.eTry.getData()
    cld := oroute.cld
    self.huntstop_scodes = oroute.huntstop_scodes
    if self.global_config.Static_tr_out != "" {
        var err error
        cld, err = re_replace(self.global_config.Static_tr_out, cld)
        if err != nil {
            self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (7)", nil, ""))
            self.state = CCStateDead
            return
        }
    }
    var nh_address *sippy_net.HostPort
    var host string
    if oroute.hostport == "sip-ua" {
        host = self.source.Host.String()
        nh_address = self.source
    } else {
        host = oroute.hostonly
        nh_address = oroute.getNHAddr(self.source)
    }
    if ! oroute.forward_on_fail && self.global_config.Acct_enable {
        self.acctO = NewRadiusAccounting(self.global_config, "originate", self.cmap.radius_client)
        bill_to := self.username
        if v, ok := oroute.params["bill-to"]; ok {
            bill_to = v
        }
        cli := oroute.cli
        if v, ok := oroute.params["bill-cli"]; ok {
            cli = v
        }
        cld := oroute.cld
        if v, ok := oroute.params["bill-cld"]; ok {
            cld = v
        }
        self.acctO.SetParams(bill_to, cli, cld, self.cGUID.StringBody(), self.cId.StringBody(), host, "")
    } else {
        self.acctO = nil
    }
    //self.acctA.credit_time = oroute.credit_time
    self.uaO = sippy.NewUA(self.sip_tm, self.global_config, nh_address, self, self.lock, nil)
    self.uaO.SetUsername(oroute.user)
    self.uaO.SetPassword(oroute.passw)
    if oroute.credit_time > 0 {
        self.uaO.SetCreditTime(oroute.credit_time)
    }
    self.uaO.SetConnCb(self.oConn)
    if ! oroute.forward_on_fail && self.global_config.Acct_enable {
        self.uaO.SetDiscCb(func(rtime *sippy_time.MonoTime, origin string, scode int, req sippy_types.SipRequest) { self.acctO.Disc(self.uaO, rtime, origin, scode) })
        self.uaO.SetFailCb(func(rtime *sippy_time.MonoTime, origin string, scode int) { self.acctO.Disc(self.uaO, rtime, origin, scode) })
    }
    self.uaO.SetDeadCb(self.oDead)
    if oroute.expires > 0 {
        self.uaO.SetExpireTime(oroute.expires)
    }
    self.uaO.SetNoProgressTime(oroute.no_progress_expires)
    extra_headers := []sippy_header.SipHeader{ self.cGUID, self.cGUID.AsH323ConfId() }
    extra_headers = append(extra_headers, oroute.extra_headers...)
    self.uaO.SetExtraHeaders(extra_headers)
    self.uaO.SetDeadCb(self.oDead)
    self.uaO.SetLocalUA(sippy_header.NewSipUserAgent(self.global_config.GetMyUAName()))
    if oroute.outbound_proxy != nil && self.source.String() != oroute.outbound_proxy.String() {
        self.uaO.SetOutboundProxy(oroute.outbound_proxy)
    }
    var body sippy_types.MsgBody
    if self.rtp_proxy_session != nil && oroute.rtpp {
        self.uaO.SetOnLocalSdpChange(self.rtp_proxy_session.OnCallerSdpChange)
        self.uaO.SetOnRemoteSdpChange(self.rtp_proxy_session.OnCalleeSdpChange)
        self.rtp_proxy_session.SetCallerRaddress(nh_address)
        if self.eTry.GetBody() != nil {
            body = self.eTry.GetBody().GetCopy()
        }
        self.proxied = true
    }
    self.uaO.SetKaInterval(self.global_config.Keepalive_orig_dur)
    if gt, ok := oroute.params["gt"]; ok {
        arr := strings.SplitN(gt, ",", 2)
        if len(arr) == 2 {
            timeout, err := strconv.Atoi(arr[0])
            if err == nil {
                var skipto int
                skipto, err = strconv.Atoi(arr[1])
                if err == nil {
                    go func() {
                        time.Sleep(time.Duration(timeout) * time.Second)
                        self.lock.Lock()
                        defer self.lock.Unlock()
                        self.group_expires(skipto)
                    }()
                }
            }
        }
    }
    var cId *sippy_header.SipCallId
    if self.global_config.Hide_call_id {
        cId = sippy_header.NewSipCallIdFromString(fmt.Sprintf("%x-b2b_%d", md5.Sum([]byte(self.eTry.GetSipCallId().CallId)), oroute.rnum))
    } else {
        cId = sippy_header.NewSipCallIdFromString(self.eTry.GetSipCallId().CallId + fmt.Sprintf("-b2b_%d", oroute.rnum))
    }
    caller_name := oroute.caller_name
    if caller_name == "" {
        caller_name = self.caller_name
    }
    event, _ := sippy.NewCCEventTry(cId, oroute.cli, cld, body, self.eTry.GetSipAuthorizationHF(), caller_name, nil, "")
    if self.eTry.GetMaxForwards() != nil {
        mf_body, err := self.eTry.GetMaxForwards().GetBody()
        max_forwards := mf_body.Number - 1
        if err != nil {
            self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (8)", nil, ""))
            self.state = CCStateDead
            return
        }
        if max_forwards <= 0 {
            self.uaA.RecvEvent(sippy.NewCCEventFail(483, "Too Many Hops", nil, ""))
            self.state = CCStateDead
            return
        }
        event.SetMaxForwards(sippy_header.NewSipMaxForwards(max_forwards))
    }
    event.SetReason(self.eTry.GetReason())
    self.uaO.RecvEvent(event)
}

func (self *callController) disconnect(rtime *sippy_time.MonoTime) {
    self.uaA.Disconnect(rtime, "")
}

func (self *callController) oConn(rtime *sippy_time.MonoTime, origin string) {
    if self.acctO != nil {
        self.acctO.Conn(self.uaO, rtime, origin)
    }
}

func (self *callController) aConn(rtime *sippy_time.MonoTime, origin string) {
    self.state = CCStateConnected
    self.acctA.Conn(self.uaA, rtime, origin)
}

func (self *callController) aFail(rtime *sippy_time.MonoTime, origin string, result int) {
    self.aDisc(rtime, origin, result, nil)
}

func (self *callController) aDisc(rtime *sippy_time.MonoTime, origin string, result int, inreq sippy_types.SipRequest) {
    if self.state == CCStateWaitRoute && self.auth_proc != nil {
        self.auth_proc.Cancel()
        self.auth_proc = nil
    }
    if self.uaO != nil && self.state != CCStateDead {
        self.state = CCStateDisconnecting
    } else {
        self.state = CCStateDead
    }
    if self.acctA != nil {
        self.acctA.Disc(self.uaA, rtime, origin, result)
    }
    if self.rtp_proxy_session != nil {
        self.rtp_proxy_session.Delete()
        self.rtp_proxy_session = nil
    }
}

func (self *callController) aDead() {
    if self.uaO == nil || self.uaO.GetState() == sippy_types.UA_STATE_DEAD {
        if self.cmap.debug_mode {
            println("garbadge collecting", self)
        }
        self.acctA = nil
        self.acctO = nil
        self.cmap.DropCC(self.id)
        self.cmap = nil
    }
}

func (self *callController) oDead() {
    if self.uaA.GetState() == sippy_types.UA_STATE_DEAD {
        if self.cmap.debug_mode {
            println("garbadge collecting", self)
        }
        self.acctA = nil
        self.acctO = nil
        self.cmap.DropCC(self.id)
    }
}

func (self *callController) group_expires(skipto int) {
    if self.state != CCStateARComplete || len(self.routes) == 0 || self.routes[0].rnum > skipto ||
      ((self.uaA.GetState() != sippy_types.UAS_STATE_TRYING) && (self.uaA.GetState() != sippy_types.UAS_STATE_RINGING)) {
        return
    }
    // When the last group in the list has timeouted don't disconnect
    // the current attempt forcefully. Instead, make sure that if the
    // current originate call leg fails no more routes will be
    // processed.
    if skipto == self.routes[len(self.routes)-1].rnum + 1 {
        self.routes = []*B2BRoute{}
        return
    }
    for self.routes[0].rnum != skipto {
        self.routes = self.routes[1:]
    }
    self.uaO.Disconnect(nil, "")
}
