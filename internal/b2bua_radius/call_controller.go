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
package b2bua_radius

import (
    "fmt"
    "strings"
    "sync"

    "github.com/sippy/go-b2bua/sippy"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

type callController struct {
    id              int64
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
    rtp_proxy_session *sippy.Rtp_proxy_session
    eTry            *sippy.CCEventTry
    huntstop_scodes []int
    acctA           *fakeAccounting
    sip_tm          sippy_types.SipTransactionManager
    proxied         bool
    sdp_session     *sippy.SdpSession
    cmap            *CallMap
}
/*
class CallController(object):
    cld = nil
    acctA = nil
    acctO = nil
    global_config = nil
    rtp_proxy_session = nil
    huntstop_scodes = nil
    auth_proc = nil
    challenge = nil
*/
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
    self.uaA.SetKaInterval(self.global_config.keepalive_ans)
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
            //, body, auth, 
            self.caller_name = ev_try.GetCallerName()
            if self.cld == "" {
                self.uaA.RecvEvent(sippy.NewCCEventFail(500, "Internal Server Error (1)", event.GetRtime(), ""))
                self.state = CCStateDead
                return
            }
/*
            if body != nil && self.global_config.has_key('_allowed_pts') {
                try:
                    body.parse()
                except:
                    self.uaA.RecvEvent(CCEventFail((400, "Malformed SDP Body"), rtime = event.rtime))
                    self.state = CCStateDead
                    return
                allowed_pts = self.global_config['_allowed_pts']
                mbody = body.content.sections[0].m_header
                if mbody.transport.lower() == "rtp/avp" {
                    old_len = len(mbody.formats)
                    mbody.formats = [x for x in mbody.formats if x in allowed_pts]
                    if len(mbody.formats) == 0 {
                        self.uaA.RecvEvent(CCEventFail((488, "Not Acceptable Here")))
                        self.state = CCStateDead
                        return
                    if old_len > len(mbody.formats) {
                        body.content.sections[0].optimize_a()
*/
            if strings.HasPrefix(self.cld, "nat-") {
                self.cld = self.cld[4:]
                if ev_try.GetBody() != nil {
                    ev_try.GetBody().AppendAHeader("nated:yes")
                }
                event, _ = sippy.NewCCEventTry(self.cId, self.cli, self.cld, ev_try.GetBody(), ev_try.GetSipAuthorizationHF(), self.caller_name, nil, "")
            }
/*
            if self.global_config.has_key('static_tr_in') {
                self.cld = re_replace(self.global_config['static_tr_in'], self.cld)
                event = sippy.NewCCEventTry(self.cId, self.cGUID, self.cli, self.cld, body, auth, self.caller_name)
            }
*/
            if len(*self.cmap.rtp_proxy_clients) > 0 {
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
            //if ! self.global_config['auth_enable'] {
                //self.username = self.remote_ip
                self.rDone()
            //} else if auth == nil || auth.username == nil || len(auth.username) == 0 {
            //    self.username = self.remote_ip
            //    self.auth_proc = self.global_config['_radius_client'].do_auth(self.remote_ip, self.cli, self.cld, self.cGUID, \
            //      self.cId, self.remote_ip, self.rDone)
            //} else {
            //    self.username = auth.username
            //    self.auth_proc = self.global_config['_radius_client'].do_auth(auth.username, self.cli, self.cld, self.cGUID, 
            //      self.cId, self.remote_ip, self.rDone, auth.realm, auth.nonce, auth.uri, auth.response)
            //}
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

func (self *callController) rDone(/*results*/) {
/*
    // Check that we got necessary result from Radius
    if len(results) != 2 || results[1] != 0:
        if isinstance(self.uaA.state, UasStateTrying):
            if self.challenge != nil:
                event = CCEventFail((401, "Unauthorized"))
                event.extra_header = self.challenge
            else:
                event = CCEventFail((403, "Auth Failed"))
            self.uaA.RecvEvent(event)
            self.state = CCStateDead
        return
    if self.global_config['acct_enable']:
        self.acctA = RadiusAccounting(self.global_config, "answer", \
          send_start = self.global_config['start_acct_enable'], lperiod = \
          self.global_config.getdefault('alive_acct_int', nil))
        self.acctA.ms_precision = self.global_config.getdefault('precise_acct', false)
        self.acctA.setParams(self.username, self.cli, self.cld, self.cGUID, self.cId, self.remote_ip)
    else:
*/
        self.acctA = NewFakeAccounting()
    // Check that uaA is still in a valid state, send acct stop
    if self.uaA.GetState() != sippy_types.UAS_STATE_TRYING {
        //self.acctA.disc(self.uaA, time(), "caller")
        return
    }
/*
    cli = [x[1][4:] for x in results[0] if x[0] == "h323-ivr-in" && x[1].startswith("CLI:")]
    if len(cli) > 0:
        self.cli = cli[0]
        if len(self.cli) == 0:
            self.cli = nil
    caller_name = [x[1][5:] for x in results[0] if x[0] == "h323-ivr-in" && x[1].startswith("CNAM:")]
    if len(caller_name) > 0:
        self.caller_name = caller_name[0]
        if len(self.caller_name) == 0:
            self.caller_name = nil
    credit_time = [x for x in results[0] if x[0] == "h323-credit-time"]
    if len(credit_time) > 0:
        credit_time = int(credit_time[0][1])
    else:
        credit_time := time.Duration(0)
    if ! self.global_config.has_key('_static_route'):
        routing = [x for x in results[0] if x[0] == "h323-ivr-in" && x[1].startswith("Routing:")]
        if len(routing) == 0:
            self.uaA.RecvEvent(CCEventFail((500, "Internal Server Error (2)")))
            self.state = CCStateDead
            return
        routing = [B2BRoute(x[1][8:]) for x in routing]
    else {
*/
        routing := []*B2BRoute{ self.cmap.static_route.getCopy() }
//    }
    rnum := 0
    for _, oroute := range routing {
        rnum += 1
        oroute.customize(rnum, self.cld, self.cli, 0, self.pass_headers, 0)
        //oroute.customize(rnum, self.cld, self.cli, credit_time, self.pass_headers, self.global_config.max_credit_time)
        //if oroute.credit_time == 0 || oroute.expires == 0 {
        //    continue
        //}
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
    //if self.global_config.has_key('static_tr_out') {
    //    cld = re_replace(self.global_config['static_tr_out'], cld)
    //}
    var nh_address *sippy_net.HostPort
    if oroute.hostport == "sip-ua" {
        //host = self.source[0]
        nh_address = self.source
    } else {
        //host = oroute.hostonly
        nh_address = oroute.getNHAddr(self.source)
    }
    //if ! oroute.forward_on_fail && self.global_config['acct_enable'] {
    //    self.acctO = RadiusAccounting(self.global_config, "originate",
    //      send_start = self.global_config['start_acct_enable'], /*lperiod*/
    //      self.global_config.getdefault('alive_acct_int', nil))
    //    self.acctO.ms_precision = self.global_config.getdefault('precise_acct', false)
    //    self.acctO.setParams(oroute.params.get('bill-to', self.username), oroute.params.get('bill-cli', oroute.cli), \
    //      oroute.params.get('bill-cld', cld), self.cGUID, self.cId, host)
    //else {
    //    self.acctO = nil
    //}
    //self.acctA.credit_time = oroute.credit_time
    //disc_handlers = []
    //if ! oroute.forward_on_fail && self.global_config['acct_enable'] {
    //    disc_handlers.append(self.acctO.disc)
    //}
    self.uaO = sippy.NewUA(self.sip_tm, self.global_config, nh_address, self, self.lock, nil)
    // oroute.user, oroute.passw, nh_address, oroute.credit_time,
    //  /*expire_time*/ oroute.expires, /*no_progress_time*/ oroute.no_progress_expires, /*extra_headers*/ oroute.extra_headers)
    //self.uaO.SetConnCbs([]sippy_types.OnConnectListener{ self.oConn })
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
    self.uaO.SetKaInterval(self.global_config.keepalive_orig)
    //if oroute.params.has_key('group_timeout') {
    //    timeout, skipto = oroute.params['group_timeout']
    //    Timeout(self.group_expires, timeout, 1, skipto)
    //}
    //if self.global_config.getdefault('hide_call_id', false) {
    //    cId = SipCallId(md5(str(cId)).hexdigest() + ("-b2b_%d" % oroute.rnum))
    //} else {
        cId := sippy_header.NewSipCallIdFromString(self.eTry.GetSipCallId().CallId + fmt.Sprintf("-b2b_%d", oroute.rnum))
    //}
    caller_name := oroute.caller_name
    if caller_name == "" {
        caller_name = self.caller_name
    }
    event, _ := sippy.NewCCEventTry(cId, oroute.cli, cld, body, self.eTry.GetSipAuthorizationHF(), caller_name, nil, "")
    //if self.eTry.max_forwards != nil {
    //    event.max_forwards = self.eTry.max_forwards - 1
    //    if event.max_forwards <= 0 {
    //        self.uaA.RecvEvent(sippy.NewCCEventFail(483, "Too Many Hops", nil, ""))
    //        self.state = CCStateDead
    //        return
    //    }
    //}
    event.SetReason(self.eTry.GetReason())
    self.uaO.RecvEvent(event)
}

func (self *callController) disconnect(rtime *sippy_time.MonoTime) {
    self.uaA.Disconnect(rtime, "")
}
/*
    def oConn(self, ua, rtime, origin):
        if self.acctO != nil:
            self.acctO.conn(ua, rtime, origin)
*/
func (self *callController) aConn(rtime *sippy_time.MonoTime, origin string) {
    self.state = CCStateConnected
    //self.acctA.conn(rtime, origin)
}

func (self *callController) aFail(rtime *sippy_time.MonoTime, origin string, result int) {
    self.aDisc(rtime, origin, result, nil)
}

func (self *callController) aDisc(rtime *sippy_time.MonoTime, origin string, result int, inreq sippy_types.SipRequest) {
    //if self.state == CCStateWaitRoute && self.auth_proc != nil {
    //    self.auth_proc.cancel()
    //    self.auth_proc = nil
    //}
    if self.uaO != nil && self.state != CCStateDead {
        self.state = CCStateDisconnecting
    } else {
        self.state = CCStateDead
    }
    //if self.acctA != nil {
    //    self.acctA.disc(ua, rtime, origin, result)
    //}
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
        //self.acctO = nil
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
        //self.acctO = nil
        self.cmap.DropCC(self.id)
    }
}

/*
    def group_expires(self, skipto):
        if self.state != CCStateARComplete || len(self.routes) == 0 || self.routes[0][0] > skipto || \
          (! isinstance(self.uaA.state, UasStateTrying) && ! isinstance(self.uaA.state, UasStateRinging)):
            return
        // When the last group in the list has timeouted don't disconnect
        // the current attempt forcefully. Instead, make sure that if the
        // current originate call leg fails no more routes will be
        // processed.
        if skipto == self.routes[-1][0] + 1:
            self.routes = []
            return
        while self.routes[0][0] != skipto:
            self.routes.pop(0)
        self.uaO.disconnect()
        */
