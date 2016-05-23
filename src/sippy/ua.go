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
    "sync"
    "time"

    "sippy/conf"
    "sippy/headers"
    "sippy/types"
    "sippy/time"
    "sippy/utils"
)

type ua struct {
    sip_tm          sippy_types.SipTransactionManager
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
    rAddr0          *sippy_conf.HostPort
    rUri            *sippy_header.SipTo
    lUri            *sippy_header.SipFrom
    ruri_userparams []string
    to_username     string
    from_domain     string
    lTag            string
    lCSeq           int
    lContact        *sippy_header.SipContact
    routes          []*sippy_header.SipRoute
    cGUID           *sippy_header.SipCiscoGUID
    h323_conf_id    *sippy_header.SipH323ConfId
    lSDP            sippy_types.MsgBody
    rSDP            sippy_types.MsgBody
    outbound_proxy  *sippy_conf.HostPort
    rAddr           *sippy_conf.HostPort
    local_ua        *sippy_header.SipUserAgent
    username        string
    password        string
    extra_headers   []sippy_header.SipHeader
    reqs            map[int]*sipRequest
    tr              sippy_types.ClientTransaction
    source_address  *sippy_conf.HostPort
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
    conn_cbs        []sippy_types.OnConnectListener
    dead_cbs        []sippy_types.OnDeadListener
    disc_cbs        []sippy_types.OnDisconnectListener
    fail_cbs        []sippy_types.OnFailureListener
    ring_cbs        []sippy_types.OnRingingListener
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
}

func (self *ua) String() string {
    return "UA state: " + self.state.String() + ", Call-Id: " + self.cId.CallId
}

func NewUA(sip_tm sippy_types.SipTransactionManager, config sippy_conf.Config, nh_address *sippy_conf.HostPort, call_controller sippy_types.CallController, session_lock sync.Locker) *ua {
    return &ua{
        sip_tm          : sip_tm,
        call_controller : call_controller,
        equeue          : make([]sippy_types.CCEvent, 0),
        elast_seq       : -1,
        ruri_userparams : nil,
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
        fail_cbs        : make([]sippy_types.OnFailureListener, 0),
        ring_cbs        : make([]sippy_types.OnRingingListener, 0),
        disc_cbs        : make([]sippy_types.OnDisconnectListener, 0),
        conn_cbs        : make([]sippy_types.OnConnectListener, 0),
        dead_cbs        : make([]sippy_types.OnDeadListener, 0),
        session_lock    : session_lock,
        pass_auth       : false,
        late_media      : false,
    }
}

func (self *ua) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) (*sippy_types.Ua_context) {
    //print "Received request %s in state %s instance %s" % (req.getMethod(), self.state, self)
    //print self.rCSeq, req.getHFBody("cseq").getCSeqNum()
    if self.remote_ua == "" {
        self.update_ua(req)
    }
    if self.rCSeq != -1 && self.rCSeq >= req.GetCSeq().CSeq {
        return &sippy_types.Ua_context{
            Response : req.GenResponse(500, "Server Internal Error", /*body*/ nil, /*server*/ self.local_ua.AsSipServer()),
            CancelCB : nil,
            NoAckCB  : nil,
        }
    }
    self.rCSeq = req.GetCSeq().CSeq
    if self.state == nil {
        if req.GetMethod() == "INVITE" {
            self.ChangeState(NewUasStateIdle(self, self.config))
        } else {
            return nil
        }
    }
    newstate := self.state.RecvRequest(req, t)
    if newstate != nil {
        self.ChangeState(newstate)
    }
    self.emitPendingEvents()
    if newstate != nil && req.GetMethod() == "INVITE" {
        return &sippy_types.Ua_context{
            Response : nil,
            CancelCB : self.state.Cancel,
            NoAckCB  : self.Disconnect,
        }
    } else {
        return nil
    }
}

func (self *ua) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) {
    var err error
    if self.state == nil {
        return
    }
    self.update_ua(resp)
    code, _ := resp.GetSCode()
    cseq, method := resp.GetCSeq().CSeq, resp.GetCSeq().Method
    orig_req, cseq_found := self.reqs[cseq]
    if method == "INVITE" && !self.pass_auth && cseq_found && code == 401 && resp.GetSipWWWAuthenticate() != nil &&
      self.username != "" && self.password != "" && orig_req.sip_authorization == nil {
        challenge := resp.GetSipWWWAuthenticate()
        req := self.GenRequest("INVITE", self.lSDP, challenge.GetNonce(), challenge.GetRealm(), /*SipXXXAuthorization*/ nil)
        self.lCSeq += 1
        self.tr, err = self.sip_tm.NewClientTransaction(req, self, self.session_lock, /*laddress*/ self.source_address, /*udp_server*/ nil)
        if err == nil {
            self.tr.SetOutboundProxy(self.outbound_proxy)
            delete(self.reqs, cseq)
        }
        return
    }
    if method == "INVITE" && !self.pass_auth && cseq_found && code == 407 && resp.GetSipProxyAuthenticate() != nil &&
      self.username != "" && self.password != "" && orig_req.GetSipProxyAuthorization() == nil {
        challenge := resp.GetSipProxyAuthenticate()
        req := self.GenRequest("INVITE", self.lSDP, challenge.GetNonce(), challenge.GetRealm(), sippy_header.NewSipProxyAuthorization)
        self.lCSeq += 1
        self.tr, err = self.sip_tm.NewClientTransaction(req, self, self.session_lock, /*laddress*/ self.source_address, /*udp_server*/ nil)
        if err == nil {
            self.tr.SetOutboundProxy(self.outbound_proxy)
        }
        delete(self.reqs, cseq)
        return
    }
    if code >= 200 && cseq_found {
        delete(self.reqs, cseq)
    }
    newstate := self.state.RecvResponse(resp, tr)
    if newstate != nil {
        self.ChangeState(newstate)
    }
    self.emitPendingEvents()
}

func (self *ua) RecvEvent(event sippy_types.CCEvent) {
    //print self, event
    if self.state == nil {
        switch event.(type) {
        case *CCEventTry:
        case *CCEventFail:
        case *CCEventDisconnect:
        default:
            return
        }
        self.ChangeState(NewUacStateIdle(self, self.config))
    }
    newstate, err := self.state.RecvEvent(event)
    if err != nil {
        self.logError("UA::RecvEvent error #1:", err)
        return
    }
    if newstate != nil {
        self.ChangeState(newstate)
    }
    self.emitPendingEvents()
}

func (self *ua) Disconnect(rtime *sippy_time.MonoTime) {
    if self.sip_tm == nil {
        return // we are already in a dead state
    }
    if rtime == nil {
        rtime, _ = sippy_time.NewMonoTime()
    }
    self.equeue = append(self.equeue, NewCCEventDisconnect(nil, rtime, ""))
    self.RecvEvent(NewCCEventDisconnect(nil, rtime, ""))
}

func (self *ua) expires() {
    self.expire_timer = nil
    self.Disconnect(nil)
}

func (self *ua) no_progress_expires() {
    self.no_progress_timer = nil
    self.Disconnect(nil)
}

func (self *ua) no_reply_expires() {
    self.no_reply_timer = nil
    self.Disconnect(nil)
}

func (self *ua) credit_expires(rtime *sippy_time.MonoTime) {
    self.credit_timer = nil
    self.Disconnect(rtime)
}

func (self *ua) ChangeState(newstate sippy_types.UaState) {
    if self.state != nil {
        self.state.OnStateChange()
    }
    self.state = newstate //.Newstate(self, self.config)
    newstate.OnActivation()
}

func (self *ua) EmitEvent(event sippy_types.CCEvent) {
    if self.call_controller != nil {
        if self.elast_seq != -1 && self.elast_seq >= event.GetSeq() {
            //print "ignoring out-of-order event", event, event.seq, self.elast_seq, self.cId
            return
        }
        self.elast_seq = event.GetSeq()
        self.call_controller.RecvEvent(event, self)
    }
}

func (self *ua) emitPendingEvents() {
    for len(self.equeue) != 0 && self.call_controller != nil {
        event := self.equeue[0]; self.equeue = self.equeue[1:]
        if self.elast_seq != -1 && self.elast_seq >= event.GetSeq() {
            //print "ignoring out-of-order event", event, event.seq, self.elast_seq, self.cId
            continue
        }
        self.elast_seq = event.GetSeq()
        self.call_controller.RecvEvent(event, self)
    }
}

func (self *ua) GenRequest(method string, body sippy_types.MsgBody, nonce string, realm string, SipXXXAuthorization sippy_header.NewSipXXXAuthorizationFunc, extra_headers ...sippy_header.SipHeader) sippy_types.SipRequest {
    var target *sippy_conf.HostPort
    if self.outbound_proxy != nil {
        target = self.outbound_proxy
    } else {
        target = self.rAddr
    }
    req := NewSipRequest(/*method*/ method, /*ruri*/ self.rTarget, /*sipver*/ "", /*to*/ self.rUri, /*fr0m*/ self.lUri,
                    /*via*/ nil, /*cseq*/ self.lCSeq, /*callid*/ self.cId, /*maxforwars*/ nil, /*body*/ body,
                    /*contact*/ self.lContact, /*routes*/ self.routes, /*target*/ target, /*cguid*/ self.cGUID,
                     /*user_agent*/ self.local_ua, /*expires*/ nil, self.config)
    if nonce != "" && realm != "" && self.username != "" && self.password != "" {
        auth := SipXXXAuthorization(/*realm*/ realm, /*nonce*/ nonce, /*method*/ method, /*uri*/ self.rTarget.String(),
          /*username*/ self.username, /*password*/ self.password)
        req.AppendHeader(auth)
    }
    if self.extra_headers != nil {
        req.appendHeaders(self.extra_headers)
    }
    if extra_headers != nil {
        req.appendHeaders(extra_headers)
    }
    self.reqs[self.lCSeq] = req
    return req
}

func (self *ua) GetUasResp() sippy_types.SipResponse {
    return self.uasResp
}

func (self *ua) SetUasResp(resp sippy_types.SipResponse) {
    self.uasResp = resp
}

func (self *ua) SendUasResponse(t sippy_types.ServerTransaction, scode int, reason string, body sippy_types.MsgBody /*= nil*/, contact *sippy_header.SipContact /*= nil*/, ack_wait bool, extra_headers ...sippy_header.SipHeader) {
    uasResp := self.uasResp.GetCopy()
    uasResp.SetSCode(scode, reason)
    uasResp.SetBody(body)
    if contact != nil {
        uasResp.AppendHeader(contact)
    }
    for _, eh := range extra_headers {
        uasResp.AppendHeader(eh)
    }
    var ack_cb func(sippy_types.SipRequest)
    if ack_wait {
        ack_cb = self.recvACK
    }
    if t != nil {
        t.SendResponse(uasResp, /*retrans*/ false, ack_cb)
    } else {
        // the lock on the server transaction is already aquired so find it but do not try to lock
        self.sip_tm.SendResponse(uasResp, /*lock*/ false, ack_cb)
    }
}

func (self *ua) recvACK(req sippy_types.SipRequest) {
    if !self.isConnected() {
        return
    }
    //print 'UA::recvACK', req
    self.state.RecvACK(req)
    self.emitPendingEvents()
}

func (self *ua) IsYours(req sippy_types.SipRequest, br0k3n_to bool /*= False*/) bool {
    //print self.branch, req.getHFBody("via").getBranch()
    if req.GetMethod() != "BYE" && self.branch != "" && self.branch != req.GetVias()[0].GetBranch() {
        return false
    }
    call_id := req.GetCallId().CallId
    from_tag := req.GetFrom().GetTag()
    to_tag := req.GetTo().GetTag()
    //print str(self.cId), call_id
    if call_id != self.cId.CallId {
        return false
    }
    //print self.rUri.getTag(), from_tag
    if self.rUri != nil && self.rUri.GetTag() != from_tag {
        return false
    }
    //print self.lUri.getTag(), to_tag
    if self.lUri != nil && self.lUri.GetTag() != to_tag && ! br0k3n_to {
        return false
    }
    return true
}

func (self *ua) DelayedRemoteSdpUpdate(event sippy_types.CCEvent, remote_sdp_body sippy_types.MsgBody) {
    self.rSDP = remote_sdp_body.GetCopy()
    self.Enqueue(event)
    self.emitPendingEvents()
}

func (self *ua) update_ua(msg sippy_types.SipMsg) {
    if msg.GetSipUserAgent() != nil {
        self.remote_ua = msg.GetSipUserAgent().UserAgent
    } else if msg.GetSipServer() != nil {
        self.remote_ua = msg.GetSipServer().Server
    }
}

func (self *ua) CancelCreditTimer() {
    //print("UA::cancelCreditTimer()")
    if self.credit_timer != nil {
        self.credit_timer.Cancel()
        self.credit_timer = nil
    }
}

func (self *ua) StartCreditTimer(rtime *sippy_time.MonoTime) {
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
    self.credit_timer = NewTimeout(func () { self.credit_expires(credit_time) }, self.session_lock, credit_time.Sub(now), 1, nil)
    self.credit_timer.Start()
}

func (self *ua) UpdateRouting(resp sippy_types.SipResponse, update_rtarget bool /*true*/, reverse_routes bool /*true*/) {
    if update_rtarget && len(resp.GetContacts()) > 0 {
        self.rTarget = resp.GetContacts()[0].GetUrl().GetCopy()
    }
    self.routes = make([]*sippy_header.SipRoute, len(resp.GetRecordRoutes()))
    for i, r := range resp.GetRecordRoutes() {
        if reverse_routes {
            self.routes[len(resp.GetRecordRoutes()) - i - 1] = r.AsSipRoute()
        } else {
            self.routes[i] = r.AsSipRoute()
        }
    }
    if len(self.routes) > 0 {
        if ! self.routes[0].GetUrl().Lr {
            self.routes = append(self.routes, sippy_header.NewSipRoute(/*address*/ sippy_header.NewSipAddress("", /*url*/ self.rTarget)))
            self.rTarget = self.routes[0].GetUrl()
            self.routes = self.routes[1:]
            self.rAddr = self.rTarget.GetAddr(self.config)
        } else {
            self.rAddr = self.routes[0].GetAddr(self.config)
        }
    } else {
        self.rAddr = self.rTarget.GetAddr(self.config)
    }
    if self.outbound_proxy != nil {
        self.routes = append([]*sippy_header.SipRoute{ sippy_header.NewSipRoute(sippy_header.NewSipAddress("", sippy_header.NewSipURL("", self.outbound_proxy.Host, self.outbound_proxy.Port, true))) }, self.routes...)
    }
}

func (self *ua) SipTM() sippy_types.SipTransactionManager {
    return self.sip_tm
}

func (self *ua) GetSetupTs() *sippy_time.MonoTime {
    return self.setup_ts
}

func (self *ua) SetSetupTs(ts *sippy_time.MonoTime) {
    self.setup_ts = ts
}

func (self *ua) GetOrigin()string {
    return self.origin
}

func (self *ua) SetOrigin(origin string) {
    self.origin = origin
}

func (self *ua) OnLocalSdpChange(body sippy_types.MsgBody, event sippy_types.CCEvent, cb func(sippy_types.MsgBody)) {
    if self.on_local_sdp_change == nil {
        return
    }
    self.on_local_sdp_change(body, event, cb)
}

func (self *ua) HasOnLocalSdpChange() bool {
    return self.on_local_sdp_change != nil
}

func (self *ua) SetCallId(call_id *sippy_header.SipCallId) {
    self.cId = call_id
}

func (self *ua) GetCallId() *sippy_header.SipCallId {
    return self.cId
}

func (self *ua) SetRTarget(url *sippy_header.SipURL) {
    self.rTarget = url
}

func (self *ua) GetRAddr0() *sippy_conf.HostPort {
    return self.rAddr0
}

func (self *ua) SetRAddr0(addr *sippy_conf.HostPort) {
    self.rAddr0 = addr
}

func (self *ua) GetRTarget() *sippy_header.SipURL {
    return self.rTarget
}

func (self *ua) SetRUri(ruri *sippy_header.SipTo) {
    self.rUri = ruri
}

func (self *ua) GetRuriUserparams() []string {
    return self.ruri_userparams
}

func (self *ua) SetRuriUserparams(ruri_userparams []string) {
    self.ruri_userparams = ruri_userparams
}

func (self *ua) GetRUri() *sippy_header.SipTo {
    return self.rUri
}

func (self *ua) GetToUsername() string {
    return self.to_username
}

func (self *ua) SetToUsername(to_username string) {
    self.to_username = to_username
}

func (self *ua) SetLUri(from *sippy_header.SipFrom) {
    self.lUri = from
}

func (self *ua) GetLUri() *sippy_header.SipFrom {
    return self.lUri
}

func (self *ua) GetFromDomain() string {
    return self.from_domain
}

func (self *ua) SetFromDomain(from_domain string) {
    self.from_domain = from_domain
}

func (self *ua) GetLTag() string {
    return self.ltag
}

func (self *ua) SetLCSeq(cseq int) {
    self.lCSeq = cseq
}

func (self *ua) GetLContact() *sippy_header.SipContact {
    return self.lContact
}

func (self *ua) SetLContact(contact *sippy_header.SipContact) {
    self.lContact = contact
}

func (self *ua) SetRoutes(routes []*sippy_header.SipRoute) {
    self.routes = routes
}

func (self *ua) GetCGUID() *sippy_header.SipCiscoGUID {
    return self.cGUID
}

func (self *ua) SetCGUID(cguid *sippy_header.SipCiscoGUID) {
    self.cGUID = cguid
}

func (self *ua) GetLSDP() sippy_types.MsgBody {
    return self.lSDP
}

func (self *ua) SetLSDP(msg sippy_types.MsgBody) {
    self.lSDP = msg
}

func (self *ua) GetRSDP() sippy_types.MsgBody {
    return self.rSDP
}

func (self *ua) SetRSDP(sdp sippy_types.MsgBody) {
    self.rSDP = sdp
}

func (self *ua) IncLCSeq() {
    self.lCSeq += 1
}

func (self *ua) GetSourceAddress() *sippy_conf.HostPort {
    return self.source_address
}

func (self *ua) SetSourceAddress(addr *sippy_conf.HostPort) {
    self.source_address = addr
}

func (self *ua) SetClientTransaction(tr sippy_types.ClientTransaction) {
    self.tr = tr
}

func (self *ua) GetClientTransaction() sippy_types.ClientTransaction {
    return self.tr
}

func (self *ua) GetOutboundProxy() *sippy_conf.HostPort {
    return self.outbound_proxy
}

func (self *ua) SetOutboundProxy(outbound_proxy *sippy_conf.HostPort) {
    self.outbound_proxy = outbound_proxy
}

func (self *ua) GetNoReplyTime() time.Duration {
    return self.no_reply_time
}

func (self *ua) SetNoReplyTime(no_reply_time time.Duration) {
    self.no_reply_time = no_reply_time
}

func (self *ua) GetExpireTime() time.Duration {
    return self.expire_time
}

func (self *ua) SetExpireTime(expire_time time.Duration) {
    self.expire_time = expire_time
}

func (self *ua) GetNoProgressTime() time.Duration {
    return self.no_progress_time
}

func (self *ua) SetNoProgressTime(no_progress_time time.Duration) {
    self.no_progress_time = no_progress_time
}

func (self *ua) StartNoReplyTimer(t *sippy_time.MonoTime) {
    now, _ := sippy_time.NewMonoTime()
    self.no_reply_timer = NewTimeout(self.no_reply_expires, self.session_lock, t.Sub(now), 1, self.config.ErrorLogger())
    self.no_reply_timer.Start()
}

func (self *ua) StartNoProgressTimer(t *sippy_time.MonoTime) {
    now, _ := sippy_time.NewMonoTime()
    self.no_progress_timer = NewTimeout(self.no_progress_expires, self.session_lock, t.Sub(now), 1, self.config.ErrorLogger())
    self.no_progress_timer.Start()
}

func (self *ua) StartExpireTimer(t *sippy_time.MonoTime) {
    now, _ := sippy_time.NewMonoTime()
    self.expire_timer = NewTimeout(self.expires, self.session_lock, t.Sub(now), 1, self.config.ErrorLogger())
    self.expire_timer.Start()
}

func (self *ua) CancelExpireTimer() {
    if self.expire_timer != nil {
        self.expire_timer.Cancel()
        self.expire_timer = nil
    }
}

func (self *ua) GetDisconnectTs() *sippy_time.MonoTime {
    return self.disconnect_ts
}

func (self *ua) SetDisconnectTs(ts *sippy_time.MonoTime) {
    self.disconnect_ts = ts
}

func (self *ua) GetDiscCbs() []sippy_types.OnDisconnectListener {
    return self.disc_cbs
}

func (self *ua) SetDiscCbs(disc_cbs []sippy_types.OnDisconnectListener) {
    self.disc_cbs = disc_cbs
}

func (self *ua) GetFailCbs() []sippy_types.OnFailureListener {
    return self.fail_cbs
}

func (self *ua) SetFailCbs(fail_cbs []sippy_types.OnFailureListener) {
    self.fail_cbs = fail_cbs
}

func (self *ua) GetDeadCbs() []sippy_types.OnDeadListener {
    return self.dead_cbs
}

func (self *ua) SetDeadCbs(dead_cbs []sippy_types.OnDeadListener) {
    self.dead_cbs = dead_cbs
}

func (self *ua) GetRAddr() *sippy_conf.HostPort {
    return self.rAddr
}

func (self *ua) SetRAddr(addr *sippy_conf.HostPort) {
    self.rAddr = addr
}

func (self *ua) OnDead() {
    if self.sip_tm == nil {
        return
    }
    if self.cId != nil {
        self.sip_tm.UnregConsumer(self, self.cId.CallId)
    }
    self.tr = nil
    self.call_controller = nil
    self.conn_cbs = make([]sippy_types.OnConnectListener, 0)
    self.fail_cbs = make([]sippy_types.OnFailureListener, 0)
    self.ring_cbs = make([]sippy_types.OnRingingListener, 0)
    self.disc_cbs = make([]sippy_types.OnDisconnectListener, 0)
    self.on_local_sdp_change = nil
    self.on_remote_sdp_change = nil
    self.expire_timer = nil
    self.no_progress_timer = nil
    self.credit_timer = nil
    // Keep this at the very end of processing
    for _, listener := range self.dead_cbs {
        listener()
    }
    self.dead_cbs = make([]sippy_types.OnDeadListener, 0)
    self.sip_tm = nil
}

func (self *ua) GetLocalUA() *sippy_header.SipUserAgent {
    return self.local_ua
}

func (self *ua) SetLocalUA(ua *sippy_header.SipUserAgent) {
    self.local_ua = ua
}

func (self *ua) Enqueue(event sippy_types.CCEvent) {
    self.equeue = append(self.equeue, event)
}

func (self *ua) OnRemoteSdpChange(body sippy_types.MsgBody, req sippy_types.SipMsg, f func(x sippy_types.MsgBody)) {
    if self.on_remote_sdp_change != nil {
        self.on_remote_sdp_change(body, req, f)
    }
}

func (self *ua) ShouldUseRefer() bool {
    return self.useRefer
}

func (self *ua) GetState() sippy_types.UaState {
    return self.state
}

func (self *ua) GetUsername() string {
    return self.username
}

func (self *ua) SetUsername(username string) {
    self.username = username
}

func (self *ua) GetPassword() string {
    return self.password
}

func (self *ua) SetPassword(passwd string) {
    self.password = passwd
}

func (self *ua) GetKaInterval() time.Duration {
    return self.kaInterval
}

func (self *ua) SetKaInterval(ka time.Duration) {
    self.kaInterval = ka
}

func (self *ua) ResetOnLocalSdpChange() {
    self.on_local_sdp_change = nil
}

func (self *ua) GetOnLocalSdpChange() sippy_types.OnLocalSdpChange {
    return self.on_local_sdp_change
}

func (self *ua) SetOnLocalSdpChange(on_local_sdp_change sippy_types.OnLocalSdpChange) {
    self.on_local_sdp_change = on_local_sdp_change
}

func (self *ua) GetOnRemoteSdpChange() sippy_types.OnRemoteSdpChange {
    return self.on_remote_sdp_change
}

func (self *ua) SetOnRemoteSdpChange(on_remote_sdp_change sippy_types.OnRemoteSdpChange) {
    self.on_remote_sdp_change = on_remote_sdp_change
}

func (self *ua) ResetOnRemoteSdpChange() {
    self.on_remote_sdp_change = nil
}

func (self *ua) GetGoDeadTimeout() time.Duration {
    return self.godead_timeout
}

func (self *ua) GetLastScode() int {
    return self.last_scode
}

func (self *ua) SetLastScode(scode int) {
    self.last_scode = scode
}

func (self *ua) HasNoReplyTimer() bool {
    return self.no_reply_timer != nil
}

func (self *ua) CancelNoReplyTimer() {
    if self.no_reply_timer != nil {
        self.no_reply_timer.Cancel()
        self.no_reply_timer = nil
    }
}

func (self *ua) GetNpMtime() *sippy_time.MonoTime {
    return self._np_mtime
}

func (self *ua) GetExMtime() *sippy_time.MonoTime {
    return self._ex_mtime
}

func (self *ua) SetExMtime(t *sippy_time.MonoTime) {
    self._ex_mtime = t
}

func (self *ua) GetP100Ts() *sippy_time.MonoTime {
    return self.p100_ts
}

func (self *ua) SetP100Ts(ts *sippy_time.MonoTime) {
    if self.p100_ts == nil {
        self.p100_ts = ts
    }
}

func (self *ua) HasNoProgressTimer() bool {
    return self.no_progress_timer != nil
}

func (self *ua) CancelNoProgressTimer() {
    if self.no_progress_timer != nil {
        self.no_progress_timer.Cancel()
        self.no_progress_timer = nil
    }
}

func (self *ua) HasOnRemoteSdpChange() bool {
    return self.on_remote_sdp_change != nil
}

func (self *ua) GetP1xxTs() *sippy_time.MonoTime {
    return self.p1xx_ts
}

func (self *ua) SetP1xxTs(ts *sippy_time.MonoTime) {
    self.p1xx_ts = ts
}

func (self *ua) GetRingCbs() []sippy_types.OnRingingListener {
    return self.ring_cbs
}

func (self *ua) GetConnectTs() *sippy_time.MonoTime {
    return self.connect_ts
}

func (self *ua) SetConnectTs(connect_ts *sippy_time.MonoTime) {
    self.connect_ts = connect_ts
}

func (self *ua) SetBranch(branch string) {
    self.branch = branch
}

func (self *ua) GetConnCbs() []sippy_types.OnConnectListener {
    return self.conn_cbs
}

func (self *ua) SetConnCbs(conn_cbs []sippy_types.OnConnectListener) {
    self.conn_cbs = conn_cbs
}

func (self *ua) SetH323ConfId(h323_conf_id *sippy_header.SipH323ConfId) {
    self.h323_conf_id = h323_conf_id
}

func (self *ua) SetAuth(auth sippy_header.SipHeader) {
    self.auth = auth
}

func (self *ua) SetNpMtime(t *sippy_time.MonoTime) {
    self._np_mtime = t
}

func (self *ua) GetNrMtime() *sippy_time.MonoTime {
    return self._nr_mtime
}

func (self *ua) SetNrMtime(t *sippy_time.MonoTime) {
    self._nr_mtime = t
}

func (self *ua) logError(args ...interface{}) {
    self.config.ErrorLogger().Error(args...)
}

func (self *ua) GetController() sippy_types.CallController {
    return self.call_controller
}

func (self *ua) SetCreditTime(credit_time time.Duration) {
    self.credit_time = &credit_time
}

func (self *ua) GetSessionLock() sync.Locker {
    return self.session_lock
}

func (self *ua) isConnected() bool {
    if self.state != nil {
        return self.state.IsConnected()
    }
    return false
}

func (self *ua) GetPendingTr() sippy_types.ClientTransaction {
    return self.pending_tr
}

func (self *ua) SetPendingTr(tr sippy_types.ClientTransaction) {
    self.pending_tr = tr
}

func (self *ua) GetLateMedia() bool {
    return self.late_media
}

func (self *ua) SetLateMedia(late_media bool) {
    self.late_media = late_media
}

func (self *ua) GetPassAuth() bool {
    return self.pass_auth
}

func (self *ua) GetRemoteUA() string {
    return self.remote_ua
}

func (self *ua) ResetCreditTime(rtime *sippy_time.MonoTime, new_credit_times map[int64]*sippy_time.MonoTime) {
    for k, v := range new_credit_times {
        self.credit_times[k] = v
    }
    if self.state.IsConnected() {
        self.CancelCreditTimer()
        self.StartCreditTimer(rtime)
    }
}

func (self *ua) SetExtraHeaders(extra_headers []sippy_header.SipHeader) {
    self.extra_headers = extra_headers
}

func (self *ua) OnUnregister() {
}

func (self *ua) GetAcct(disconnect_ts *sippy_time.MonoTime) (duration time.Duration, delay time.Duration, connected bool, disconnected bool) {
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
