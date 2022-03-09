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
    "errors"
    "strings"
    "sync"
    "time"

    "sippy/conf"
    "sippy/headers"
    "sippy/net"
    "sippy/time"
    "sippy/types"
    "sippy/utils"
)

type Ua struct {
    sip_tm          sippy_types.SipTransactionManager
    sip_tm_lock     sync.RWMutex
    config          sippy_conf.Config
    call_controller sippy_types.CallController
    session_lock    sync.Locker
    state           sippy_types.UaState
    equeue          []sippy_types.CCEvent
    elast_seq       int64
    setup_ts        *sippy_time.MonoTime
    connect_ts      *sippy_time.MonoTime
    disconnect_ts   *sippy_time.MonoTime
    origin          string
    on_local_sdp_change sippy_types.OnLocalSdpChange
    on_remote_sdp_change sippy_types.OnRemoteSdpChange
    cId             *sippy_header.SipCallId
    rTarget         *sippy_header.SipURL
    rAddr0          *sippy_net.HostPort
    rUri            *sippy_header.SipTo
    lUri            *sippy_header.SipFrom
    lTag            string
    lCSeq           int
    lContact        *sippy_header.SipContact
    routes          []*sippy_header.SipRoute
    lSDP            sippy_types.MsgBody
    rSDP            sippy_types.MsgBody
    outbound_proxy  *sippy_net.HostPort
    rAddr           *sippy_net.HostPort
    local_ua        *sippy_header.SipUserAgent
    username        string
    password        string
    extra_headers   []sippy_header.SipHeader
    dlg_headers     []sippy_header.SipHeader
    reqs            map[int]*sipRequest
    tr              sippy_types.ClientTransaction
    source_address  *sippy_net.HostPort
    remote_ua       string
    expire_time     time.Duration
    expire_timer    *Timeout
    no_progress_time time.Duration
    no_reply_time   time.Duration
    no_reply_timer  *Timeout
    no_progress_timer *Timeout
    ltag            string
    rCSeq           int
    branch          string
    conn_cb         sippy_types.OnConnectListener
    dead_cb         sippy_types.OnDeadListener
    disc_cb         sippy_types.OnDisconnectListener
    fail_cb         sippy_types.OnFailureListener
    ring_cb         sippy_types.OnRingingListener
    credit_timer    *Timeout
    uasResp         sippy_types.SipResponse
    useRefer        bool
    kaInterval      time.Duration
    godead_timeout  time.Duration
    last_scode      int
    _np_mtime       *sippy_time.MonoTime
    _nr_mtime       *sippy_time.MonoTime
    _ex_mtime       *sippy_time.MonoTime
    p100_ts         *sippy_time.MonoTime
    p1xx_ts         *sippy_time.MonoTime
    credit_time     *time.Duration
    credit_times    map[int64]*sippy_time.MonoTime
    auth            sippy_header.SipHeader
    pass_auth       bool
    pending_tr      sippy_types.ClientTransaction
    late_media      bool
    heir            sippy_types.UA
    uas_lossemul    int
    on_uac_setup_complete   func()
    expire_starts_on_setup  bool
    pr_rel          bool
    auth_enalgs     map[string]bool
}

func (self *Ua) me() sippy_types.UA {
    if self.heir == nil {
        return self
    }
    return self.heir
}

func (self *Ua) UasLossEmul() int {
    return self.uas_lossemul
}

func (self *Ua) String() string {
    return "UA state: " + self.state.String() + ", Call-Id: " + self.cId.CallId
}

func NewUA(sip_tm sippy_types.SipTransactionManager, config sippy_conf.Config, nh_address *sippy_net.HostPort, call_controller sippy_types.CallController, session_lock sync.Locker, heir sippy_types.UA) *Ua {
    return &Ua{
        sip_tm          : sip_tm,
        call_controller : call_controller,
        equeue          : make([]sippy_types.CCEvent, 0),
        elast_seq       : -1,
        reqs            : make(map[int]*sipRequest),
        rCSeq           : -1,
        useRefer        : true,
        kaInterval      : 0,
        godead_timeout  : time.Duration(32 * time.Second),
        last_scode      : 100,
        p100_ts         : nil,
        p1xx_ts         : nil,
        credit_times    : make(map[int64]*sippy_time.MonoTime),
        config          : config,
        rAddr           : nh_address,
        rAddr0          : nh_address,
        ltag            : sippy_utils.GenTag(),
        //fail_cb         : nil,
        //ring_cb         : nil,
        //disc_cb         : nil,
        //conn_cb         : nil,
        //dead_cb         : nil,
        session_lock    : session_lock,
        pass_auth       : false,
        late_media      : false,
        heir            : heir,
        expire_starts_on_setup : true,
        pr_rel          : false,
    }
}

func (self *Ua) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) *sippy_types.Ua_context {
    //print "Received request %s in state %s instance %s" % (req.getMethod(), self.state, self)
    //print self.rCSeq, req.getHFBody("cseq").getCSeqNum()
    t.SetBeforeResponseSent(self.me().BeforeResponseSent)
    if self.remote_ua == "" {
        self.update_ua(req)
    }
    cseq_body, err := req.GetCSeq().GetBody()
    if err != nil || (self.rCSeq != -1 && self.rCSeq >= cseq_body.CSeq) {
        return &sippy_types.Ua_context{
            Response : req.GenResponse(500, "Server Internal Error", /*body*/ nil, /*server*/ self.local_ua.AsSipServer()),
            CancelCB : nil,
            NoAckCB  : nil,
        }
    }
    self.rCSeq = cseq_body.CSeq
    if self.state == nil {
        if req.GetMethod() == "INVITE" {
            if req.GetBody() == nil {
                found := false
REQ_LOOP:
                for _, sip_req_str := range req.GetSipRequire() {
                    for _, it := range strings.Split(sip_req_str.StringBody(), ",") {
                        if strings.TrimSpace(it) == "100rel" {
                            found = true
                            break REQ_LOOP
                        }
                    }
                }
                if found {
                    resp := req.GenResponse(420, "Bad Extension", /*body*/ nil, /*server*/ self.local_ua.AsSipServer())
                    usup := sippy_header.NewSipGenericHF("Unsupported", "100rel")
                    resp.AppendHeader(usup)
                    return &sippy_types.Ua_context{
                        Response : resp,
                        CancelCB : nil,
                        NoAckCB  : nil,
                    }
                }
            } else {
                t.Setup100rel(req)
            }
            self.pr_rel = t.PrRel()
            self.me().ChangeState(NewUasStateIdle(self.me(), self.config), nil)
        } else {
            return nil
        }
    }
    newstate, cb := self.state.RecvRequest(req, t)
    if newstate != nil {
        self.me().ChangeState(newstate, cb)
    }
    self.emitPendingEvents()
    if newstate != nil && req.GetMethod() == "INVITE" {
        disc_fn := func(rtime *sippy_time.MonoTime) { self.me().Disconnect(rtime, "") }
        if self.pr_rel {
            t.SetPrackCBs(self.RecvPRACK, disc_fn)
        }
        return &sippy_types.Ua_context{
            Response : nil,
            CancelCB : self.state.RecvCancel,
            NoAckCB  : disc_fn,
        }
    } else {
        return nil
    }
}

func (self *Ua) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) {
    var err error
    var cseq_body *sippy_header.SipCSeqBody

    if self.state == nil {
        return
    }
    cseq_body, err = resp.GetCSeq().GetBody()
    if err != nil {
        self.logError("UA::RecvResponse: cannot parse CSeq: " + err.Error())
        return
    }
    self.update_ua(resp)
    code, _ := resp.GetSCode()
    orig_req, cseq_found := self.reqs[cseq_body.CSeq]
    if cseq_body.Method == "INVITE" && !self.pass_auth && cseq_found {
        if code == 401 && self.processWWWChallenge(resp, cseq_body.CSeq, orig_req) {
            return
        } else if code == 407 && self.processProxyChallenge(resp, cseq_body.CSeq, orig_req) {
            return
        }
    }
    if code >= 200 && cseq_found {
        delete(self.reqs, cseq_body.CSeq)
    }
    newstate, cb := self.state.RecvResponse(resp, tr)
    if newstate != nil {
        self.me().ChangeState(newstate, cb)
    }
    self.emitPendingEvents()
}

func (self *Ua) PrepTr(req sippy_types.SipRequest) (sippy_types.ClientTransaction, error) {
    sip_tm := self.get_sip_tm()
    if sip_tm == nil {
        return nil, errors.New("UA already dead")
    }
    tr, err := sip_tm.CreateClientTransaction(req, self.me(), self.session_lock, /*laddress*/ self.source_address, /*udp_server*/ nil, self.me().BeforeRequestSent)
    if err != nil {
        return nil, err
    }
    tr.SetOutboundProxy(self.outbound_proxy)
    if self.isConnected() {
        routes := make([]*sippy_header.SipRoute, len(self.routes))
        copy(routes, self.routes)
        if self.outbound_proxy == nil {
            tr.SetAckRparams(self.rAddr, self.rTarget, routes)
        } else {
            tr.SetAckRparams(self.outbound_proxy, self.rTarget, routes)
        }
    }
    return tr, nil
}

func (self *Ua) RecvEvent(event sippy_types.CCEvent) {
    //print self, event
    if self.state == nil {
        switch event.(type) {
        case *CCEventTry:
        case *CCEventFail:
        case *CCEventDisconnect:
        default:
            return
        }
        self.me().ChangeState(NewUacStateIdle(self.me(), self.config), nil)
    }
    newstate, cb, err := self.state.RecvEvent(event)
    if err != nil {
        self.logError("UA::RecvEvent error #1: " + err.Error())
        return
    }
    if newstate != nil {
        self.me().ChangeState(newstate, cb)
    }
    self.emitPendingEvents()
}

func (self *Ua) Disconnect(rtime *sippy_time.MonoTime, origin string) {
    sip_tm := self.get_sip_tm()
    if sip_tm == nil {
        return // we are already in a dead state
    }
    if rtime == nil {
        rtime, _ = sippy_time.NewMonoTime()
    }
    self.equeue = append(self.equeue, NewCCEventDisconnect(nil, rtime, origin))
    self.RecvEvent(NewCCEventDisconnect(nil, rtime, origin))
}

func (self *Ua) expires() {
    self.expire_timer = nil
    self.me().Disconnect(nil, "")
}

func (self *Ua) no_progress_expires() {
    self.no_progress_timer = nil
    self.me().Disconnect(nil, "")
}

func (self *Ua) no_reply_expires() {
    self.no_reply_timer = nil
    self.me().Disconnect(nil, "")
}

func (self *Ua) credit_expires(rtime *sippy_time.MonoTime) {
    self.credit_timer = nil
    self.me().Disconnect(rtime, "")
}

func (self *Ua) ChangeState(newstate sippy_types.UaState, cb func()) {
    if self.state != nil {
        self.state.OnDeactivate()
    }
    self.state = newstate //.Newstate(self, self.config)
    if newstate != nil {
        newstate.OnActivation()
        if cb != nil {
            cb()
        }
    }
}

func (self *Ua) EmitEvent(event sippy_types.CCEvent) {
    if self.call_controller != nil {
        if self.elast_seq != -1 && self.elast_seq >= event.GetSeq() {
            //print "ignoring out-of-order event", event, event.seq, self.elast_seq, self.cId
            return
        }
        self.elast_seq = event.GetSeq()
        sippy_utils.SafeCall(func() { self.call_controller.RecvEvent(event, self.me()) }, nil, self.config.ErrorLogger())
    }
}

func (self *Ua) emitPendingEvents() {
    for len(self.equeue) != 0 && self.call_controller != nil {
        event := self.equeue[0]; self.equeue = self.equeue[1:]
        if self.elast_seq != -1 && self.elast_seq >= event.GetSeq() {
            //print "ignoring out-of-order event", event, event.seq, self.elast_seq, self.cId
            continue
        }
        self.elast_seq = event.GetSeq()
        sippy_utils.SafeCall(func() { self.call_controller.RecvEvent(event, self.me()) }, nil, self.config.ErrorLogger())
    }
}

func (self *Ua) GenRequest(method string, body sippy_types.MsgBody, challenge sippy_types.Challenge, extra_headers ...sippy_header.SipHeader) (sippy_types.SipRequest, error) {
    var target *sippy_net.HostPort

    if self.outbound_proxy != nil {
        target = self.outbound_proxy
    } else {
        target = self.rAddr
    }
    req, err := NewSipRequest(method, /*ruri*/ self.rTarget, /*sipver*/ "", /*to*/ self.rUri, /*fr0m*/ self.lUri,
                    /*via*/ nil, self.lCSeq, self.cId, /*maxforwars*/ nil, body, self.lContact, self.routes,
                    target, /*user_agent*/ self.local_ua, /*expires*/ nil, self.config)
    if err != nil {
        return nil, err
    }
    if challenge != nil {
        entity_body := ""
        if body != nil {
            entity_body = body.String()
        }
        auth, err := challenge.GenAuthHF(self.username, self.password, method, self.rTarget.String(), entity_body)
        if err == nil {
            req.AppendHeader(auth)
        }
    }
    if self.extra_headers != nil {
        req.appendHeaders(self.extra_headers)
    }
    if extra_headers != nil {
        req.appendHeaders(extra_headers)
    }
    if self.dlg_headers != nil {
        req.appendHeaders(self.dlg_headers)
    }
    self.reqs[self.lCSeq] = req
    self.lCSeq++
    return req, nil
}

func (self *Ua) GetUasResp() sippy_types.SipResponse {
    return self.uasResp
}

func (self *Ua) SetUasResp(resp sippy_types.SipResponse) {
    self.uasResp = resp
}

func (self *Ua) SendUasResponse(t sippy_types.ServerTransaction, scode int, reason string, body sippy_types.MsgBody /*= nil*/, contacts []*sippy_header.SipContact /*= nil*/, ack_wait bool, extra_headers ...sippy_header.SipHeader) {
    uasResp := self.uasResp.GetCopy()
    uasResp.SetSCode(scode, reason)
    uasResp.SetBody(body)
    if contacts != nil {
        for _, contact := range contacts {
            uasResp.AppendHeader(contact)
        }
    }
    for _, eh := range extra_headers {
        uasResp.AppendHeader(eh)
    }
    var ack_cb func(sippy_types.SipRequest)
    if ack_wait {
        ack_cb = self.me().RecvACK
    }
    if t != nil {
        t.SendResponseWithLossEmul(uasResp, /*retrans*/ false, ack_cb, self.uas_lossemul)
    } else {
        // the lock on the server transaction is already aquired so find it but do not try to lock
        if sip_tm := self.get_sip_tm(); sip_tm != nil {
            sip_tm.SendResponseWithLossEmul(uasResp, /*lock*/ false, ack_cb, self.uas_lossemul)
        }
    }
}

func (self *Ua) RecvACK(req sippy_types.SipRequest) {
    if !self.isConnected() {
        return
    }
    //print 'UA::recvACK', req
    self.state.RecvACK(req)
    self.emitPendingEvents()
}

func (self *Ua) IsYours(req sippy_types.SipRequest, br0k3n_to bool /*= False*/) bool {
    var via0 *sippy_header.SipViaBody
    var err error

    via0, err = req.GetVias()[0].GetBody()
    if err != nil {
        self.logError("UA::IsYours error #1: " + err.Error())
        return false
    }
    //print self.branch, req.getHFBody("via").getBranch()
    if req.GetMethod() != "BYE" && self.branch != "" && self.branch != via0.GetBranch() {
        return false
    }
    call_id := req.GetCallId().CallId
    //print str(self.cId), call_id
    if call_id != self.cId.CallId {
        return false
    }
    //print self.rUri.getTag(), from_tag
    if self.rUri != nil {
        var rUri *sippy_header.SipAddress
        var from_body *sippy_header.SipAddress

        from_body, err = req.GetFrom().GetBody(self.config)
        if err != nil {
            self.logError("UA::IsYours error #2: " + err.Error())
            return false
        }
        rUri, err = self.rUri.GetBody(self.config)
        if err != nil {
            self.logError("UA::IsYours error #4: " + err.Error())
            return false
        }
        if rUri.GetTag() != from_body.GetTag() {
            return false
        }
    }
    //print self.lUri.getTag(), to_tag
    if self.lUri != nil && ! br0k3n_to {
        var lUri *sippy_header.SipAddress
        var to_body *sippy_header.SipAddress

        to_body, err = req.GetTo().GetBody(self.config)
        if err != nil {
            self.logError("UA::IsYours error #3: " + err.Error())
            return false
        }
        lUri, err = self.lUri.GetBody(self.config)
        if err != nil {
            self.logError("UA::IsYours error #5: " + err.Error())
            return false
        }
        if lUri.GetTag() != to_body.GetTag() {
            return false
        }
    }
    return true
}

func (self *Ua) DelayedRemoteSdpUpdate(event sippy_types.CCEvent, remote_sdp_body sippy_types.MsgBody) {
    self.rSDP = remote_sdp_body.GetCopy()
    self.me().Enqueue(event)
    self.emitPendingEvents()
}

func (self *Ua) update_ua(msg sippy_types.SipMsg) {
    if msg.GetSipUserAgent() != nil {
        self.remote_ua = msg.GetSipUserAgent().UserAgent
    } else if msg.GetSipServer() != nil {
        self.remote_ua = msg.GetSipServer().Server
    }
}

func (self *Ua) CancelCreditTimer() {
    //print("UA::cancelCreditTimer()")
    if self.credit_timer != nil {
        self.credit_timer.Cancel()
        self.credit_timer = nil
    }
}

func (self *Ua) StartCreditTimer(rtime *sippy_time.MonoTime) {
    //print("UA::startCreditTimer()")
    if self.credit_time != nil {
        self.credit_times[0] = rtime.Add(*self.credit_time)
        self.credit_time = nil
    }
    //credit_time = min([x for x in self.credit_times.values() if x != nil])
    var credit_time *sippy_time.MonoTime = nil
    for _, t := range self.credit_times {
        if credit_time == nil || (*credit_time).After(t) {
            credit_time = t
        }
    }
    if credit_time == nil {
        return
    }
    // TODO make use of the mono time properly
    now, _ := sippy_time.NewMonoTime()
    self.credit_timer = StartTimeout(func () { self.credit_expires(credit_time) }, self.session_lock, credit_time.Sub(now), 1, self.config.ErrorLogger())
}

func (self *Ua) UpdateRouting(resp sippy_types.SipResponse, update_rtarget bool /*true*/, reverse_routes bool /*true*/) {
    if update_rtarget && len(resp.GetContacts()) > 0 {
        contact, err := resp.GetContacts()[0].GetBody(self.config)
        if err != nil {
            self.logError("UA::UpdateRouting: error #1: " + err.Error())
            return
        }
        self.rTarget = contact.GetUrl().GetCopy()
    }
    self.routes = make([]*sippy_header.SipRoute, len(resp.GetRecordRoutes()))
    for i, r := range resp.GetRecordRoutes() {
        if reverse_routes {
            self.routes[len(resp.GetRecordRoutes()) - i - 1] = r.AsSipRoute()
        } else {
            self.routes[i] = r.AsSipRoute()
        }
    }
    if self.outbound_proxy != nil {
        self.routes = append([]*sippy_header.SipRoute{ sippy_header.NewSipRoute(sippy_header.NewSipAddress("", sippy_header.NewSipURL("", self.outbound_proxy.Host, self.outbound_proxy.Port, true))) }, self.routes...)
    }
    if len(self.routes) > 0 {
        r0, err := self.routes[0].GetBody(self.config)
        if err != nil {
            self.logError("UA::UpdateRouting: error #2: " + err.Error())
            return
        }
        if ! r0.GetUrl().Lr {
            self.routes = append(self.routes, sippy_header.NewSipRoute(/*address*/ sippy_header.NewSipAddress("", /*url*/ self.rTarget)))
            self.rTarget = r0.GetUrl()
            self.routes = self.routes[1:]
            self.rAddr = self.rTarget.GetAddr(self.config)
        } else {
            self.rAddr = r0.GetUrl().GetAddr(self.config)
        }
    } else {
        self.rAddr = self.rTarget.GetAddr(self.config)
    }
}

func (self *Ua) GetSetupTs() *sippy_time.MonoTime {
    return self.setup_ts
}

func (self *Ua) SetSetupTs(ts *sippy_time.MonoTime) {
    self.setup_ts = ts
}

func (self *Ua) GetOrigin()string {
    return self.origin
}

func (self *Ua) SetOrigin(origin string) {
    self.origin = origin
}

func (self *Ua) OnLocalSdpChange(body sippy_types.MsgBody, cb func(sippy_types.MsgBody)) error {
    if self.on_local_sdp_change == nil {
        return nil
    }
    return self.on_local_sdp_change(body, cb)
}

func (self *Ua) HasOnLocalSdpChange() bool {
    return self.on_local_sdp_change != nil
}

func (self *Ua) SetCallId(call_id *sippy_header.SipCallId) {
    self.cId = call_id
}

func (self *Ua) GetCallId() *sippy_header.SipCallId {
    return self.cId
}

func (self *Ua) SetRTarget(url *sippy_header.SipURL) {
    self.rTarget = url
}

func (self *Ua) GetRAddr0() *sippy_net.HostPort {
    return self.rAddr0
}

func (self *Ua) SetRAddr0(addr *sippy_net.HostPort) {
    self.rAddr0 = addr
}

func (self *Ua) GetRTarget() *sippy_header.SipURL {
    return self.rTarget
}

func (self *Ua) SetRUri(ruri *sippy_header.SipTo) {
    self.rUri = ruri
}

func (self *Ua) GetRUri() *sippy_header.SipTo {
    return self.rUri
}

func (self *Ua) SetLUri(from *sippy_header.SipFrom) {
    self.lUri = from
}

func (self *Ua) GetLUri() *sippy_header.SipFrom {
    return self.lUri
}

func (self *Ua) GetLTag() string {
    return self.ltag
}

func (self *Ua) SetLCSeq(cseq int) {
    self.lCSeq = cseq
}

func (self *Ua) GetLContact() *sippy_header.SipContact {
    return self.lContact
}

func (self *Ua) GetLContacts() []*sippy_header.SipContact {
    contact := self.lContact // copy the value into a local variable for thread safety
    if contact == nil {
        return nil
    }
    return []*sippy_header.SipContact{ contact }
}

func (self *Ua) SetLContact(contact *sippy_header.SipContact) {
    self.lContact = contact
}

func (self *Ua) SetRoutes(routes []*sippy_header.SipRoute) {
    self.routes = routes
}

func (self *Ua) GetLSDP() sippy_types.MsgBody {
    return self.lSDP
}

func (self *Ua) SetLSDP(msg sippy_types.MsgBody) {
    self.lSDP = msg
}

func (self *Ua) GetRSDP() sippy_types.MsgBody {
    return self.rSDP
}

func (self *Ua) SetRSDP(sdp sippy_types.MsgBody) {
    self.rSDP = sdp
}

func (self *Ua) GetSourceAddress() *sippy_net.HostPort {
    return self.source_address
}

func (self *Ua) SetSourceAddress(addr *sippy_net.HostPort) {
    self.source_address = addr
}

func (self *Ua) SetClientTransaction(tr sippy_types.ClientTransaction) {
    self.tr = tr
}

func (self *Ua) GetClientTransaction() sippy_types.ClientTransaction {
    return self.tr
}

func (self *Ua) GetOutboundProxy() *sippy_net.HostPort {
    return self.outbound_proxy
}

func (self *Ua) SetOutboundProxy(outbound_proxy *sippy_net.HostPort) {
    self.outbound_proxy = outbound_proxy
}

func (self *Ua) GetNoReplyTime() time.Duration {
    return self.no_reply_time
}

func (self *Ua) SetNoReplyTime(no_reply_time time.Duration) {
    self.no_reply_time = no_reply_time
}

func (self *Ua) GetExpireTime() time.Duration {
    return self.expire_time
}

func (self *Ua) SetExpireTime(expire_time time.Duration) {
    self.expire_time = expire_time
}

func (self *Ua) GetNoProgressTime() time.Duration {
    return self.no_progress_time
}

func (self *Ua) SetNoProgressTime(no_progress_time time.Duration) {
    self.no_progress_time = no_progress_time
}

func (self *Ua) StartNoReplyTimer() {
    now, _ := sippy_time.NewMonoTime()
    self.no_reply_timer = StartTimeout(self.no_reply_expires, self.session_lock, self._nr_mtime.Sub(now), 1, self.config.ErrorLogger())
}

func (self *Ua) StartNoProgressTimer() {
    now, _ := sippy_time.NewMonoTime()
    self.no_progress_timer = StartTimeout(self.no_progress_expires, self.session_lock, self._np_mtime.Sub(now), 1, self.config.ErrorLogger())
}

func (self *Ua) StartExpireTimer(start *sippy_time.MonoTime) {
    var d time.Duration
    now, _ := sippy_time.NewMonoTime()
    if self.expire_starts_on_setup {
        d = self._ex_mtime.Sub(now)
    } else {
        d = self.expire_time - now.Sub(start)
    }
    self.expire_timer = StartTimeout(self.expires, self.session_lock, d, 1, self.config.ErrorLogger())
}

func (self *Ua) CancelExpireTimer() {
    if self.expire_timer != nil {
        self.expire_timer.Cancel()
        self.expire_timer = nil
    }
}

func (self *Ua) GetDisconnectTs() *sippy_time.MonoTime {
    return self.disconnect_ts
}

func (self *Ua) SetDisconnectTs(ts *sippy_time.MonoTime) {
    if self.connect_ts != nil && self.connect_ts.After(ts) {
        self.disconnect_ts = self.connect_ts
    } else {
        self.disconnect_ts = ts
    }
}

func (self *Ua) DiscCb(rtime *sippy_time.MonoTime, origin string, scode int, inreq sippy_types.SipRequest) {
    if disc_cb := self.disc_cb; disc_cb != nil {
        disc_cb(rtime, origin, scode, inreq)
    }
}

func (self *Ua) GetDiscCb() sippy_types.OnDisconnectListener {
    return self.disc_cb
}

func (self *Ua) SetDiscCb(disc_cb sippy_types.OnDisconnectListener) {
    self.disc_cb = disc_cb
}

func (self *Ua) FailCb(rtime *sippy_time.MonoTime, origin string, scode int) {
    if fail_cb := self.fail_cb; fail_cb != nil {
        fail_cb(rtime, origin, scode)
    }
}

func (self *Ua) GetFailCb() sippy_types.OnFailureListener {
    return self.fail_cb
}

func (self *Ua) SetFailCb(fail_cb sippy_types.OnFailureListener) {
    self.fail_cb = fail_cb
}

func (self *Ua) GetDeadCb() sippy_types.OnDeadListener {
    return self.dead_cb
}

func (self *Ua) SetDeadCb(dead_cb sippy_types.OnDeadListener) {
    self.dead_cb = dead_cb
}

func (self *Ua) GetRAddr() *sippy_net.HostPort {
    return self.rAddr
}

func (self *Ua) SetRAddr(addr *sippy_net.HostPort) {
    self.rAddr = addr
}

func (self *Ua) OnDead() {
    self.sip_tm_lock.Lock()
    defer self.sip_tm_lock.Unlock()
    if self.sip_tm == nil {
        return
    }
    if self.cId != nil {
        self.sip_tm.UnregConsumer(self.me(), self.cId.CallId)
    }
    self.tr = nil
    self.call_controller = nil
    self.conn_cb = nil
    self.fail_cb = nil
    self.ring_cb = nil
    self.disc_cb = nil
    self.on_local_sdp_change = nil
    self.on_remote_sdp_change = nil
    self.expire_timer = nil
    self.no_progress_timer = nil
    self.credit_timer = nil
    // Keep this at the very end of processing
    if self.dead_cb != nil {
        self.dead_cb()
    }
    self.dead_cb = nil
    self.sip_tm = nil
}

func (self *Ua) GetLocalUA() *sippy_header.SipUserAgent {
    return self.local_ua
}

func (self *Ua) SetLocalUA(ua *sippy_header.SipUserAgent) {
    self.local_ua = ua
}

func (self *Ua) Enqueue(event sippy_types.CCEvent) {
    self.equeue = append(self.equeue, event)
}

func (self *Ua) OnRemoteSdpChange(body sippy_types.MsgBody, f func(x sippy_types.MsgBody)) error {
    if self.on_remote_sdp_change != nil {
        return self.on_remote_sdp_change(body, f)
    }
    return nil
}

func (self *Ua) ShouldUseRefer() bool {
    return self.useRefer
}

func (self *Ua) GetStateName() string {
    if state := self.state; state != nil {
        return state.String()
    }
    return "None"
}

func (self *Ua) GetState() sippy_types.UaStateID {
    if state := self.state; state != nil {
        return state.ID()
    }
    return sippy_types.UA_STATE_NONE
}

func (self *Ua) GetUsername() string {
    return self.username
}

func (self *Ua) SetUsername(username string) {
    self.username = username
}

func (self *Ua) GetPassword() string {
    return self.password
}

func (self *Ua) SetPassword(passwd string) {
    self.password = passwd
}

func (self *Ua) GetKaInterval() time.Duration {
    return self.kaInterval
}

func (self *Ua) SetKaInterval(ka time.Duration) {
    self.kaInterval = ka
}

func (self *Ua) ResetOnLocalSdpChange() {
    self.on_local_sdp_change = nil
}

func (self *Ua) GetOnLocalSdpChange() sippy_types.OnLocalSdpChange {
    return self.on_local_sdp_change
}

func (self *Ua) SetOnLocalSdpChange(on_local_sdp_change sippy_types.OnLocalSdpChange) {
    self.on_local_sdp_change = on_local_sdp_change
}

func (self *Ua) GetOnRemoteSdpChange() sippy_types.OnRemoteSdpChange {
    return self.on_remote_sdp_change
}

func (self *Ua) SetOnRemoteSdpChange(on_remote_sdp_change sippy_types.OnRemoteSdpChange) {
    self.on_remote_sdp_change = on_remote_sdp_change
}

func (self *Ua) ResetOnRemoteSdpChange() {
    self.on_remote_sdp_change = nil
}

func (self *Ua) GetGoDeadTimeout() time.Duration {
    return self.godead_timeout
}

func (self *Ua) GetLastScode() int {
    return self.last_scode
}

func (self *Ua) SetLastScode(scode int) {
    self.last_scode = scode
}

func (self *Ua) HasNoReplyTimer() bool {
    return self.no_reply_timer != nil
}

func (self *Ua) CancelNoReplyTimer() {
    if self.no_reply_timer != nil {
        self.no_reply_timer.Cancel()
        self.no_reply_timer = nil
    }
}

func (self *Ua) GetNpMtime() *sippy_time.MonoTime {
    return self._np_mtime
}

func (self *Ua) GetExMtime() *sippy_time.MonoTime {
    return self._ex_mtime
}

func (self *Ua) SetExMtime(t *sippy_time.MonoTime) {
    self._ex_mtime = t
}

func (self *Ua) GetP100Ts() *sippy_time.MonoTime {
    return self.p100_ts
}

func (self *Ua) SetP100Ts(ts *sippy_time.MonoTime) {
    if self.p100_ts == nil {
        self.p100_ts = ts
    }
}

func (self *Ua) HasNoProgressTimer() bool {
    return self.no_progress_timer != nil
}

func (self *Ua) CancelNoProgressTimer() {
    if self.no_progress_timer != nil {
        self.no_progress_timer.Cancel()
        self.no_progress_timer = nil
    }
}

func (self *Ua) HasOnRemoteSdpChange() bool {
    return self.on_remote_sdp_change != nil
}

func (self *Ua) GetP1xxTs() *sippy_time.MonoTime {
    return self.p1xx_ts
}

func (self *Ua) SetP1xxTs(ts *sippy_time.MonoTime) {
    self.p1xx_ts = ts
}

func (self *Ua) RingCb(rtime *sippy_time.MonoTime, origin string, scode int) {
    if ring_cb := self.ring_cb; ring_cb != nil {
        ring_cb(rtime, origin, scode)
    }
}

func (self *Ua) GetConnectTs() *sippy_time.MonoTime {
    return self.connect_ts
}

func (self *Ua) SetConnectTs(connect_ts *sippy_time.MonoTime) {
    if self.connect_ts == nil {
        if self.disconnect_ts != nil && connect_ts.After(self.disconnect_ts) {
            self.connect_ts = self.disconnect_ts
        } else {
            self.connect_ts = connect_ts
        }
    }
}

func (self *Ua) SetBranch(branch string) {
    self.branch = branch
}

func (self *Ua) ConnCb(rtime *sippy_time.MonoTime, origin string) {
    if conn_cb := self.conn_cb; conn_cb != nil {
        conn_cb(rtime, origin)
    }
}

func (self *Ua) GetConnCb() sippy_types.OnConnectListener {
    return self.conn_cb
}

func (self *Ua) SetConnCb(conn_cb sippy_types.OnConnectListener) {
    self.conn_cb = conn_cb
}

func (self *Ua) SetAuth(auth sippy_header.SipHeader) {
    self.auth = auth
}

func (self *Ua) SetNpMtime(t *sippy_time.MonoTime) {
    self._np_mtime = t
}

func (self *Ua) GetNrMtime() *sippy_time.MonoTime {
    return self._nr_mtime
}

func (self *Ua) SetNrMtime(t *sippy_time.MonoTime) {
    self._nr_mtime = t
}

func (self *Ua) logError(args ...interface{}) {
    self.config.ErrorLogger().Error(args...)
}

func (self *Ua) GetController() sippy_types.CallController {
    return self.call_controller
}

func (self *Ua) SetCreditTime(credit_time time.Duration) {
    self.credit_time = &credit_time
}

func (self *Ua) GetSessionLock() sync.Locker {
    return self.session_lock
}

func (self *Ua) isConnected() bool {
    if self.state != nil {
        return self.state.IsConnected()
    }
    return false
}

func (self *Ua) GetPendingTr() sippy_types.ClientTransaction {
    return self.pending_tr
}

func (self *Ua) SetPendingTr(tr sippy_types.ClientTransaction) {
    self.pending_tr = tr
}

func (self *Ua) GetLateMedia() bool {
    return self.late_media
}

func (self *Ua) SetLateMedia(late_media bool) {
    self.late_media = late_media
}

func (self *Ua) GetPassAuth() bool {
    return self.pass_auth
}

func (self *Ua) GetRemoteUA() string {
    return self.remote_ua
}

func (self *Ua) ResetCreditTime(rtime *sippy_time.MonoTime, new_credit_times map[int64]*sippy_time.MonoTime) {
    for k, v := range new_credit_times {
        self.credit_times[k] = v
    }
    if self.state.IsConnected() {
        self.me().CancelCreditTimer()
        self.me().StartCreditTimer(rtime)
    }
}

func (self *Ua) GetExtraHeaders() []sippy_header.SipHeader {
    return self.extra_headers
}

func (self *Ua) SetExtraHeaders(extra_headers []sippy_header.SipHeader) {
    self.extra_headers = extra_headers
}

func (self *Ua) OnUnregister() {
}

func (self *Ua) GetAcct(disconnect_ts *sippy_time.MonoTime) (duration time.Duration, delay time.Duration, connected bool, disconnected bool) {
    if self.disconnect_ts != nil {
        disconnect_ts = self.disconnect_ts
        disconnected = true
    } else {
        if disconnect_ts == nil {
            disconnect_ts, _ = sippy_time.NewMonoTime()
        }
        disconnected = false
    }
    if self.connect_ts != nil {
        duration = disconnect_ts.Sub(self.connect_ts)
        delay = self.connect_ts.Sub(self.setup_ts)
        connected = true
        return
    }
    duration = 0
    delay = disconnect_ts.Sub(self.setup_ts)
    connected = false
    return
}

func (self *Ua) GetCLD() string {
    if self.rUri == nil {
        return ""
    }
    rUri, err := self.rUri.GetBody(self.config)
    if err != nil {
        self.logError("UA::GetCLD: " + err.Error())
        return ""
    }
    return rUri.GetUrl().Username
}

func (self *Ua) GetCLI() string {
    if self.lUri == nil {
        return ""
    }
    lUri, err := self.lUri.GetBody(self.config)
    if err != nil {
        self.logError("UA::GetCLI: " + err.Error())
        return ""
    }
    return lUri.GetUrl().Username
}

func (self *Ua) GetUasLossEmul() int {
    return 0
}

func (self *Ua) Config() sippy_conf.Config {
    return self.config
}

func (self *Ua) BeforeResponseSent(sippy_types.SipResponse) {
}

func (self *Ua) BeforeRequestSent(sippy_types.SipRequest) {
}

func (self *Ua) OnUacSetupComplete() {
    if self.on_uac_setup_complete != nil {
        self.on_uac_setup_complete()
    }
}

func (self *Ua) SetOnUacSetupComplete(fn func()) {
    self.on_uac_setup_complete = fn
}

func (self *Ua) Cleanup() {
}

func (self *Ua) OnEarlyUasDisconnect(ev sippy_types.CCEvent) (int, string) {
    return 500, "Disconnected"
}

func (self *Ua) SetExpireStartsOnSetup(v bool) {
    self.expire_starts_on_setup = v
}

func (self *Ua) RecvPRACK(req sippy_types.SipRequest, resp sippy_types.SipResponse) {
    state := self.state
    if state != nil {
        state.RecvPRACK(req, resp)
    }
}

func (self *Ua) PrRel() bool {
    return self.pr_rel
}

func (self *Ua) processProxyChallenge(resp sippy_types.SipResponse, cseq int, orig_req sippy_types.SipRequest) bool {
    if self.username == "" || self.password == "" || orig_req.GetSipProxyAuthorization() != nil {
        return false
    }
    auths := resp.GetSipProxyAuthenticates()
    challenges := make([]sippy_types.Challenge, len(auths))
    for i, hdr := range auths {
        challenges[i] = hdr
    }
    return self.processChallenge(challenges, cseq)
}

func (self *Ua) processWWWChallenge(resp sippy_types.SipResponse, cseq int, orig_req sippy_types.SipRequest) bool {
    if self.username == "" || self.password == "" || orig_req.GetSipAuthorization() != nil {
        return false
    }
    auths := resp.GetSipWWWAuthenticates()
    challenges := make([]sippy_types.Challenge, len(auths))
    for i, hdr := range auths {
        challenges[i] = hdr
    }
    return self.processChallenge(challenges, cseq)
}

func (self *Ua) processChallenge(challenges []sippy_types.Challenge, cseq int) bool {
    var challenge sippy_types.Challenge
    found := false
    for _, challenge = range challenges {
        algorithm, err := challenge.Algorithm()
        if err != nil {
            self.logError("UA::processChallenge: cannot get algorithm: " + err.Error())
            return false
        }
        if self.auth_enalgs != nil {
            if _, ok := self.auth_enalgs[algorithm]; ! ok {
                continue
            }
        }
        supported, err := challenge.SupportedAlgorithm()
        if err == nil && supported {
            found = true
            break
        }
    }
    if ! found {
        return false
    }
    if challenge == nil {
        // no supported challenge has been found
        return false
    }
    req, err := self.GenRequest("INVITE", self.lSDP, challenge)
    if err != nil {
        self.logError("UA::processChallenge: cannot create INVITE: " + err.Error())
        return false
    }
    self.lCSeq += 1
    self.tr, err = self.me().PrepTr(req)
    if err != nil {
        self.logError("UA::processChallenge: cannot prepare client transaction: " + err.Error())
        return false
    }
    self.tr.SetDlgHeaders(self.dlg_headers)
    self.BeginClientTransaction(req, self.tr)
    delete(self.reqs, cseq)
    return true
}

func (self *Ua) PassAuth() bool {
    return self.pass_auth
}

func (self *Ua) get_sip_tm() sippy_types.SipTransactionManager {
    self.sip_tm_lock.RLock()
    defer self.sip_tm_lock.RUnlock()
    return self.sip_tm
}

func (self *Ua) BeginClientTransaction(req sippy_types.SipRequest, tr sippy_types.ClientTransaction) {
    sip_tm := self.get_sip_tm()
    if sip_tm == nil {
        return
    }
    sip_tm.BeginClientTransaction(req, tr)
}

func (self *Ua) BeginNewClientTransaction(req sippy_types.SipRequest, resp_receiver sippy_types.ResponseReceiver) {
    sip_tm := self.get_sip_tm()
    if sip_tm == nil {
        return
    }
    sip_tm.BeginNewClientTransaction(req, resp_receiver, self.session_lock, self.source_address, nil /*userv*/, self.me().BeforeRequestSent)
}

func (self *Ua) RegConsumer(consumer sippy_types.UA, call_id string) {
    sip_tm := self.get_sip_tm()
    if sip_tm == nil {
        return
    }
    sip_tm.RegConsumer(consumer, call_id)
}

func (self *Ua) GetDlgHeaders() []sippy_header.SipHeader {
    return self.dlg_headers
}

func (self *Ua) SetDlgHeaders(hdrs []sippy_header.SipHeader) {
    self.dlg_headers = hdrs
}

func (self *Ua) OnReinvite(req sippy_types.SipRequest, event_update sippy_types.CCEvent) {
}
