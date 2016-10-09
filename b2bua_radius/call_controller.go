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
    "sync"

    "sippy"
    "sippy/conf"
    "sippy/headers"
    "sippy/types"
)

type callController struct {
    id              int64
    uaA             sippy_types.UA
    uaO             sippy_types.UA
    global_config   *myConfigParser
    state           CCState
    remote_ip       string
    source          *sippy_conf.HostPort
    routes          []*B2BRoute
    pass_headers    []sippy_header.SipHeader
    lock            sync.Mutex
}
/*
class CallController(object):
    cId = nil
    cld = nil
    eTry = nil
    acctA = nil
    acctO = nil
    global_config = nil
    rtp_proxy_session = nil
    huntstop_scodes = nil
    auth_proc = nil
    proxied = false
    challenge = nil
*/
func NewCallController(id int64, remote_ip string, source *sippy_conf.HostPort, global_config *myConfigParser, pass_headers []sippy_header.SipHeader, sip_tm sippy_types.SipTransactionManager) *callController {
    self := &callController{
        id              : id,
        global_config   : global_config,
        state           : CCStateIdle,
        remote_ip       : remote_ip,
        source          : source,
        routes          : make([]*B2BRoute, 0),
        pass_headers    : pass_headers,
    }
    self.uaA = sippy.NewUA(sip_tm, global_config, nil, self, self.lock, nil)
    self.uaA.kaInterval = self.global_config.keepalive_ans
    self.uaA.local_ua = self.global_config.GetMyUAName()
    self.uaA.SetConnCbs([]sippy_types.OnConnectListener{ self.aConn})
    self.uaA.SetDiscCbs([]sippy_types.OnDisconnectListener{ self.aDisc })
    self.uaA.SetFailCbs([]sippy_types.OnFailureListener{ self.aDisc })
    self.uaA.SetDeadCbs([]sippy_types.OnDeadListener{ self.aDead })
}

func (self *callController) RecvEvent(event sippy_types.CCEvent, ua sippy_types.UA) {
    if ua == self.uaA {
        if self.state == CCStateIdle {
            if ! isinstance(event, CCEventTry) {
                // Some weird event received
                self.uaA.RecvEvent(sippy.NewCCEventDisconnect(event.rtime))
                return
            }
            self.cId, cGUID, self.cli, self.cld, body, auth, self.caller_name = event.getData()
            self.cGUID = cGUID.hexForm()
            if self.cld == nil {
                self.uaA.recvEvent(sippy.NewCCEventFail(500, "Internal Server Error (1)", event.rtime))
                self.state = CCStateDead
                return
            }
/*
            if body != nil && self.global_config.has_key('_allowed_pts') {
                try:
                    body.parse()
                except:
                    self.uaA.recvEvent(CCEventFail((400, "Malformed SDP Body"), rtime = event.rtime))
                    self.state = CCStateDead
                    return
                allowed_pts = self.global_config['_allowed_pts']
                mbody = body.content.sections[0].m_header
                if mbody.transport.lower() == "rtp/avp" {
                    old_len = len(mbody.formats)
                    mbody.formats = [x for x in mbody.formats if x in allowed_pts]
                    if len(mbody.formats) == 0 {
                        self.uaA.recvEvent(CCEventFail((488, "Not Acceptable Here")))
                        self.state = CCStateDead
                        return
                    if old_len > len(mbody.formats) {
                        body.content.sections[0].optimize_a()
*/
            if self.cld.startswith("nat-") {
                self.cld = self.cld[4:]
                if body != nil {
                    body.content += "a=nated:yes\r\n"
                }
                event = sippy.NewCCEventTry(self.cId, cGUID, self.cli, self.cld, body, auth, self.caller_name)
            }
/*
            if self.global_config.has_key('static_tr_in') {
                self.cld = re_replace(self.global_config['static_tr_in'], self.cld)
                event = sippy.NewCCEventTry(self.cId, cGUID, self.cli, self.cld, body, auth, self.caller_name)
            }
*/
            if len(global_rtp_proxy_clients) > 0 {
                self.rtp_proxy_session = sippy.NewRtp_proxy_session(self.global_config, self.cId, /*notify_socket*/ self.global_config.b2bua_socket, /*notify_tag*/ url.QueryEscape(fmt.Sprint("r %d", self.id)))
                self.rtp_proxy_session.callee.raddress = sippy_conf.NewHostPort(self.remote_ip, "5060")
                self.rtp_proxy_session.insert_nortpp = true
            }
            self.eTry = event
            self.state = CCStateWaitRoute
            //if ! self.global_config['auth_enable'] {
                self.username = self.remote_ip
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
        self.uaO.recvEvent(event)
    } else {
        if (isinstance(event, CCEventFail) || isinstance(event, CCEventDisconnect)) && self.state == CCStateARComplete &&
          (isinstance(self.uaA.state, UasStateTrying) || isinstance(self.uaA.state, UasStateRinging)) && len(self.routes) > 0 {
            if isinstance(event, CCEventFail) {
                code = event.getData()[0]
            } else {
                code = nil
            }
            if code == nil || code ! in self.huntstop_scodes {
                self.placeOriginate(self.routes.pop(0))
                return
            }
        }
        self.uaA.recvEvent(event)
    }
}

/*
    def rDone(self, results):
        // Check that we got necessary result from Radius
        if len(results) != 2 || results[1] != 0:
            if isinstance(self.uaA.state, UasStateTrying):
                if self.challenge != nil:
                    event = CCEventFail((401, "Unauthorized"))
                    event.extra_header = self.challenge
                else:
                    event = CCEventFail((403, "Auth Failed"))
                self.uaA.recvEvent(event)
                self.state = CCStateDead
            return
        if self.global_config['acct_enable']:
            self.acctA = RadiusAccounting(self.global_config, "answer", \
              send_start = self.global_config['start_acct_enable'], lperiod = \
              self.global_config.getdefault('alive_acct_int', nil))
            self.acctA.ms_precision = self.global_config.getdefault('precise_acct', false)
            self.acctA.setParams(self.username, self.cli, self.cld, self.cGUID, self.cId, self.remote_ip)
        else:
            self.acctA = FakeAccounting()
        // Check that uaA is still in a valid state, send acct stop
        if ! isinstance(self.uaA.state, UasStateTrying):
            self.acctA.disc(self.uaA, time(), "caller")
            return
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
            credit_time = nil
        if ! self.global_config.has_key('_static_route'):
            routing = [x for x in results[0] if x[0] == "h323-ivr-in" && x[1].startswith("Routing:")]
            if len(routing) == 0:
                self.uaA.recvEvent(CCEventFail((500, "Internal Server Error (2)")))
                self.state = CCStateDead
                return
            routing = [B2BRoute(x[1][8:]) for x in routing]
        else:
            routing = [self.global_config['_static_route'].getCopy(),]
        rnum = 0
        for oroute in routing:
            rnum += 1
            max_credit_time = self.global_config.getdefault('max_credit_time', nil)
            oroute.customize(rnum, self.cld, self.cli, credit_time, self.pass_headers, \
              max_credit_time)
            if oroute.credit_time == 0 || oroute.expires == 0:
                continue
            self.routes.append(oroute)
            //print "Got route:", oroute.hostport, oroute.cld
        if len(self.routes) == 0:
            self.uaA.recvEvent(CCEventFail((500, "Internal Server Error (3)")))
            self.state = CCStateDead
            return
        self.state = CCStateARComplete
        self.placeOriginate(self.routes.pop(0))

    def placeOriginate(self, oroute):
        cId, cGUID, cli, cld, body, auth, caller_name = self.eTry.getData()
        cld = oroute.cld
        self.huntstop_scodes = oroute.huntstop_scodes
        if self.global_config.has_key('static_tr_out'):
            cld = re_replace(self.global_config['static_tr_out'], cld)
        if oroute.hostport == "sip-ua":
            host = self.source[0]
            nh_address, same_af = self.source, true
        else:
            host = oroute.hostonly
            nh_address, same_af = oroute.getNHAddr(self.source)
        if ! oroute.forward_on_fail && self.global_config['acct_enable']:
            self.acctO = RadiusAccounting(self.global_config, "originate", \
              send_start = self.global_config['start_acct_enable'], lperiod = \
              self.global_config.getdefault('alive_acct_int', nil))
            self.acctO.ms_precision = self.global_config.getdefault('precise_acct', false)
            self.acctO.setParams(oroute.params.get('bill-to', self.username), oroute.params.get('bill-cli', oroute.cli), \
              oroute.params.get('bill-cld', cld), self.cGUID, self.cId, host)
        else:
            self.acctO = nil
        self.acctA.credit_time = oroute.credit_time
        conn_handlers = [self.oConn]
        disc_handlers = []
        if ! oroute.forward_on_fail && self.global_config['acct_enable']:
            disc_handlers.append(self.acctO.disc)
        self.uaO = UA(self.global_config, self.recvEvent, oroute.user, oroute.passw, nh_address, oroute.credit_time, tuple(conn_handlers), \
          tuple(disc_handlers), tuple(disc_handlers), dead_cbs = (self.oDead,), expire_time = oroute.expires, \
          no_progress_time = oroute.no_progress_expires, extra_headers = oroute.extra_headers)
        self.uaO.local_ua = self.global_config.GetMyUAName()
        if self.source != oroute.outbound_proxy {
            self.uaO.outbound_proxy = oroute.outbound_proxy
        }
        if self.rtp_proxy_session != nil && oroute.params.get('rtpp', true):
            self.uaO.on_local_sdp_change = self.rtp_proxy_session.on_caller_sdp_change
            self.uaO.on_remote_sdp_change = self.rtp_proxy_session.on_callee_sdp_change
            self.rtp_proxy_session.caller.raddress = nh_address
            if body != nil:
                body = body.getCopy()
            self.proxied = true
        self.uaO.kaInterval = self.global_config['keepalive_orig']
        if oroute.params.has_key('group_timeout'):
            timeout, skipto = oroute.params['group_timeout']
            Timeout(self.group_expires, timeout, 1, skipto)
        if self.global_config.getdefault('hide_call_id', false):
            cId = SipCallId(md5(str(cId)).hexdigest() + ("-b2b_%d" % oroute.rnum))
        else:
            cId += "-b2b_%d" % oroute.rnum
        event = CCEventTry((cId, cGUID, oroute.cli, cld, body, auth, \
          oroute.params.get('caller_name', self.caller_name)))
        if self.eTry.max_forwards != nil:
            event.max_forwards = self.eTry.max_forwards - 1
            if event.max_forwards <= 0:
                self.uaA.recvEvent(CCEventFail((483, "Too Many Hops")))
                self.state = CCStateDead
                return
        event.reason = self.eTry.reason
        self.uaO.recvEvent(event)
*/
func (self *callController) disconnect() {
    self.uaA.Disconnect(nil)
}
/*
    def oConn(self, ua, rtime, origin):
        if self.acctO != nil:
            self.acctO.conn(ua, rtime, origin)

    def aConn(self, ua, rtime, origin):
        self.state = CCStateConnected
        self.acctA.conn(ua, rtime, origin)

    def aDisc(self, ua, rtime, origin, result = 0):
        if self.state == CCStateWaitRoute && self.auth_proc != nil:
            self.auth_proc.cancel()
            self.auth_proc = nil
        if self.uaO != nil && self.state != CCStateDead:
            self.state = CCStateDisconnecting
        else:
            self.state = CCStateDead
        if self.acctA != nil:
            self.acctA.disc(ua, rtime, origin, result)
        self.rtp_proxy_session = nil

    def aDead(self, ua):
        if (self.uaO == nil || isinstance(self.uaO.state, UaStateDead)):
            if global_cmap.debug_mode:
                print("garbadge collecting", self)
            self.acctA = nil
            self.acctO = nil
            global_cmap.ccmap.remove(self)

    def oDead(self, ua):
        if ua == self.uaO && isinstance(self.uaA.state, UaStateDead):
            if global_cmap.debug_mode:
                print("garbadge collecting", self)
            self.acctA = nil
            self.acctO = nil
            global_cmap.ccmap.remove(self)

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
