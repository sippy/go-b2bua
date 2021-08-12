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

    "sippy/headers"
    "sippy/net"
    "sippy/time"
    "sippy/types"
)

type provInFlight struct {
    t       *Timeout
    rtid    *sippy_header.RTID
}

type serverTransaction struct {
    *baseTransaction
    lock            sync.Mutex
    checksum        string
    teD             *Timeout
    teF             *Timeout
    teE             *Timeout
    r487            sippy_types.SipResponse
    cancel_cb       func(*sippy_time.MonoTime, sippy_types.SipRequest)
    method          string
    server          *sippy_header.SipServer
    noack_cb        func(*sippy_time.MonoTime)
    branch          string
    session_lock    sync.Locker
    expires         time.Duration
    ack_cb          func(sippy_types.SipRequest)
    before_response_sent func(sippy_types.SipResponse)
    prov_inflight   *provInFlight
    prov_inflight_lock sync.Mutex
    prack_cb        func(sippy_types.SipRequest, sippy_types.SipResponse)
    noprack_cb      func(*sippy_time.MonoTime)
    rseq            *sippy_header.SipRSeq
    pr_rel          bool
}

func NewServerTransaction(req sippy_types.SipRequest, checksum string, tid *sippy_header.TID, userv sippy_net.Transport, sip_tm *sipTransactionManager) (sippy_types.ServerTransaction, error) {
    needack := false
    var r487 sippy_types.SipResponse
    var branch string
    var expires time.Duration = 0
    method := req.GetMethod()
    if method == "INVITE" {
        via0, err := req.GetVias()[0].GetBody()
        if err != nil {
            return nil, err
        }
        needack = true
        r487 = req.GenResponse(487, "Request Terminated", /*body*/ nil, /*server*/ nil)
        branch = via0.GetBranch()
        expires = 300 * time.Second
        if req.GetExpires() != nil && req.GetExpires().Number > 0 {
            expires = time.Duration(req.GetExpires().Number) * time.Second
        }
    }
    self := &serverTransaction{
        method          : method,
        checksum        : checksum,
        r487            : r487,
        branch          : branch,
        expires         : expires,
        pr_rel          : false,
        //prov_inflight   : nil,
        rseq            : sippy_header.NewSipRSeq(),
    }
    self.baseTransaction = newBaseTransaction(self, tid, userv, sip_tm, nil, nil, needack)
    return self, nil
}

func (self *serverTransaction) StartTimers() {
    if self.expires > 0 {
        self.startTeE(self.expires)
    }
}

func (self *serverTransaction) Cleanup() {
    self.cleanup()
}

func (self *serverTransaction) cleanup() {
    self.baseTransaction.cleanup()
    self.r487 = nil
    self.cancel_cb = nil
    if self.teD != nil { self.teD.Cancel(); self.teD = nil }
    if self.teE != nil { self.teE.Cancel(); self.teE = nil }
    if self.teF != nil { self.teF.Cancel(); self.teF = nil }
    self.noack_cb = nil
    self.ack_cb = nil
    self.prack_cb = nil
    self.noprack_cb = nil
}

func (self *serverTransaction) startTeE(t time.Duration) {
    self.teE = StartTimeout(self.timerE, self, t, 1, self.logger)
}

func (self *serverTransaction) startTeF(t time.Duration) {
    if self.teF != nil {
        self.teF.Cancel()
    }
    self.teF = StartTimeout(self.timerF, self, t, 1, self.logger)
}

func (self *serverTransaction) cancelTeE() {
    if self.teE != nil {
        self.teE.Cancel()
        self.teE = nil
    }
}

func (self *serverTransaction) cancelTeD() {
    if self.teD != nil {
        self.teD.Cancel()
        self.teD = nil
    }
}

func (self *serverTransaction) cancelTeF() {
    if self.teF != nil {
        self.teF.Cancel()
        self.teF = nil
    }
}

func (self *serverTransaction) startTeD() {
    if self.teD != nil {
        self.teD.Cancel()
    }
    self.teD = StartTimeout(self.timerD, self, 32 * time.Second, 1, self.logger)
}

func (self *serverTransaction) timerD() {
    sip_tm := self.sip_tm
    if sip_tm == nil {
        return
    }
    //print("timerD", t, t.GetTID())
    if self.noack_cb != nil && self.state != CONFIRMED {
        self.noack_cb(nil)
    }
    sip_tm.tserver_del(self.tid)
    self.cleanup()
}

func (self *serverTransaction) timerE() {
    if self.sip_tm == nil {
        return
    }
    //print("timerE", t.GetTID())
    self.cancelTeE()
    self.cancelTeF()
    if self.state == TRYING || self.state == RINGING {
        if self.r487 != nil {
            self.r487.SetSCodeReason("Request Expired")
        }
        self.doCancel(/*rtime*/nil, /*req*/nil)
    }
}

// Timer to retransmit the last provisional reply every
// 2 seconds
func (self *serverTransaction) timerF() {
    sip_tm := self.sip_tm
    if sip_tm == nil {
        return
    }
    //print("timerF", t.GetTID())
    self.cancelTeF()
    if self.state == RINGING && sip_tm.provisional_retr > 0 {
        sip_tm.transmitData(self.userv, self.data, self.address, /*checksum*/ "", self.tid.CallId, 0)
        self.startTeF(sip_tm.provisional_retr)
    }
}

func (self *serverTransaction) SetCancelCB(cancel_cb func(*sippy_time.MonoTime, sippy_types.SipRequest)) {
    self.cancel_cb = cancel_cb
}

func (self *serverTransaction) SetNoackCB(noack_cb func(*sippy_time.MonoTime)) {
    self.noack_cb = noack_cb
}

func (self *serverTransaction) doCancel(rtime *sippy_time.MonoTime, req sippy_types.SipRequest) {
    if rtime == nil {
        rtime, _ = sippy_time.NewMonoTime()
    }
    if self.r487 != nil {
        self.SendResponse(self.r487, true, nil)
    }
    self.cancel_cb(rtime, req)
}

func (self *serverTransaction) IncomingRequest(req sippy_types.SipRequest, checksum string) {
    sip_tm := self.sip_tm
    if sip_tm == nil {
        return
    }
    //println("existing transaction")
    switch req.GetMethod() {
    case self.method:
        // Duplicate received, check that we have sent any response on this
        // request already
        if self.data != nil && len(self.data) > 0 {
            sip_tm.transmitData(self.userv, self.data, self.address, checksum, self.tid.CallId, 0)
        }
    case "CANCEL":
        // RFC3261 says that we have to reply 200 OK in all cases if
        // there is such transaction
        resp := req.GenResponse(200, "OK", /*body*/ nil, self.server)
        via0, err := resp.GetVias()[0].GetBody()
        if err != nil {
            self.logger.Debug("error parsing Via: " + err.Error())
            return
        }
        sip_tm.transmitMsg(self.userv, resp, via0.GetTAddr(sip_tm.config), checksum, self.tid.CallId)
        if self.state == TRYING || self.state == RINGING {
            self.doCancel(req.GetRtime(), req)
        }
    case "ACK":
        if self.state == COMPLETED {
            self.state = CONFIRMED
            self.cancelTeA()
            self.cancelTeD()
            if self.ack_cb != nil {
                self.ack_cb(req)
            }
            // We have done with the transaction, no need to wait for timeout
            sip_tm.tserver_del(self.tid)
            sip_tm.rcache_set_call_id(checksum, self.tid.CallId)
            self.cleanup()
        }
    case "PRACK":
        var resp sippy_types.SipResponse
        rskey, err := req.GetRTId()
        if err != nil {
            self.logger.Debug("Cannot get rtid: " + err.Error())
            return
        }
        self.prov_inflight_lock.Lock()
        if self.prov_inflight != nil && *self.prov_inflight.rtid == *rskey {
            self.prov_inflight.t.Cancel()
            self.prov_inflight = nil
            self.prov_inflight_lock.Unlock()
            sip_tm.rtid_del(rskey)
            resp = req.GenResponse(200, "OK", nil /*body*/, nil /*server*/)
            self.prack_cb(req, resp)
        } else {
            self.prov_inflight_lock.Unlock()
            //print('rskey: %s, prov_inflight: %s' % (str(rskey), str(self.prov_inflight)))
            resp = req.GenResponse(481, "Huh?", nil /*body*/, nil /*server*/)
        }
        via0, err := resp.GetVias()[0].GetBody()
        if err != nil {
            self.logger.Debug("error parsing Via: " + err.Error())
            return
        }
        sip_tm.transmitMsg(self.userv, resp, via0.GetTAddr(sip_tm.config), checksum, self.tid.CallId)
    }
}

func (self *serverTransaction) SendResponse(resp sippy_types.SipResponse, retrans bool, ack_cb func(sippy_types.SipRequest)) {
    self.SendResponseWithLossEmul(resp, retrans, ack_cb, 0)
}

func (self *serverTransaction) SendResponseWithLossEmul(resp sippy_types.SipResponse, retrans bool, ack_cb func(sippy_types.SipRequest), lossemul int) {
    var via0 *sippy_header.SipViaBody
    var err error

    sip_tm := self.sip_tm
    if sip_tm == nil {
        return
    }
    if self.state != TRYING && self.state != RINGING && ! retrans {
        self.logger.Error("BUG: attempt to send reply on already finished transaction!!!")
    }
    scode := resp.GetSCodeNum()
    if scode > 100 {
        to, err := resp.GetTo().GetBody(sip_tm.config)
        if err != nil {
            self.logger.Debug("error parsing To: " + err.Error())
            return
        }
        if to.GetTag() == "" {
            to.GenTag()
        }
    }
    if self.pr_rel && scode > 100 && scode < 200 {
        rseq := self.rseq.GetCopy()
        rseq_body, err := self.rseq.GetBody()
        if err != nil {
            self.logger.Debug("error parsing RSeq: " + err.Error())
            return
        }
        rseq_body.Number++
        resp.AppendHeader(rseq)
        resp.AppendHeader(sippy_header.CreateSipRequire("100rel")[0])
        tid, err := resp.GetTId(false /*wCSM*/, true /*wBRN*/, false /*wTTG*/)
        if err != nil {
            self.logger.Debug("Cannot get tid: " + err.Error())
            return
        }
        rtid, err := resp.GetRTId()
        if err != nil {
            self.logger.Debug("Cannot get rtid: " + err.Error())
            return
        }
        if lossemul > 0 {
            lossemul -= 1
        }
        self.prov_inflight_lock.Lock()
        if self.prov_inflight == nil {
            timeout := 500 * time.Millisecond
            self.prov_inflight = &provInFlight{
                t       : StartTimeout(func() { self.retrUasResponse(timeout, lossemul) }, self, timeout, 1, self.logger),
                rtid    : rtid,
            }
        } else {
            self.logger.Error("Attempt to start new PRACK timeout while the old one is still active")
        }
        self.prov_inflight_lock.Unlock()
        sip_tm.rtid_put(rtid, tid)
    }
    sip_tm.beforeResponseSent(resp)
    self.data = []byte(resp.LocalStr(self.userv.GetLAddress(), /*compact*/ false))
    via0, err = resp.GetVias()[0].GetBody()
    if err != nil {
        self.logger.Debug("error parsing Via: " + err.Error())
        return
    }
    self.address = via0.GetTAddr(sip_tm.config)
    need_cleanup := false
    if resp.GetSCodeNum() < 200 {
        self.state = RINGING
        if sip_tm.provisional_retr > 0 && resp.GetSCodeNum() > 100 {
            self.startTeF(sip_tm.provisional_retr)
        }
    } else {
        self.state = COMPLETED
        self.cancelTeE()
        self.cancelTeF()
        if self.needack {
            // Schedule removal of the transaction
            self.ack_cb = ack_cb
            self.startTeD()
            if resp.GetSCodeNum() >= 200 {
                // Black magick to allow proxy send us another INVITE
                // same branch and From tag. Use To tag to match
                // ACK transaction after this point. Branch tag in ACK
                // could differ as well.
                tid := self.tid
                if tid != nil {
                    to, err := resp.GetTo().GetBody(sip_tm.config)
                    if err != nil {
                        self.logger.Debug("error parsing To: " + err.Error())
                        return
                    }
                    old_tid := *tid // copy
                    tid.Branch = ""
                    tid.ToTag = to.GetTag()
                    self.prov_inflight_lock.Lock()
                    if self.prov_inflight != nil {
                        sip_tm.rtid_replace(self.prov_inflight.rtid, &old_tid, tid)
                    }
                    sip_tm.tserver_replace(&old_tid, tid, self)
                    self.prov_inflight_lock.Unlock()
                }
            }
            // Install retransmit timer if necessary
            self.tout = time.Duration(0.5 * float64(time.Second))
            self.startTeA()
        } else {
            // We have done with the transaction
            sip_tm.tserver_del(self.tid)
            need_cleanup = true
        }
    }
    if self.before_response_sent != nil {
        self.before_response_sent(resp)
    }
    sip_tm.transmitData(self.userv, self.data, self.address, self.checksum, self.tid.CallId, lossemul)
    if need_cleanup {
        self.cleanup()
    }
}

func (self *serverTransaction) TimersAreActive() bool {
    return self.teA != nil || self.teD != nil || self.teE != nil || self.teF != nil
}

func (self *serverTransaction) Lock() {
    self.lock.Lock()
    if self.session_lock != nil {
        self.session_lock.Lock()
        self.lock.Unlock()
    }
}

func (self *serverTransaction) Unlock() {
    if self.session_lock != nil {
        self.session_lock.Unlock()
    } else {
        self.lock.Unlock()
    }
}

func (self *serverTransaction) UpgradeToSessionLock(session_lock sync.Locker) {
    // Must be called with the self.lock already locked!
    // Must be called once only!
    self.session_lock = session_lock
    session_lock.Lock()
    self.lock.Unlock()
}

func (self *serverTransaction) SetServer(server *sippy_header.SipServer) {
    self.server = server
}

func (self *serverTransaction) SetBeforeResponseSent(cb func(resp sippy_types.SipResponse)) {
    self.before_response_sent = cb
}

func (self *serverTransaction) retrUasResponse(last_timeout time.Duration, lossemul int) {
    if last_timeout > 16 * time.Second {
        self.prov_inflight_lock.Lock()
        prov_inflight := self.prov_inflight
        self.prov_inflight = nil
        self.prov_inflight_lock.Unlock()
        if sip_tm := self.sip_tm; sip_tm != nil && prov_inflight != nil {
            sip_tm.rtid_del(prov_inflight.rtid)
        }
        self.noprack_cb(nil)
        return
    }
    if sip_tm := self.sip_tm; sip_tm != nil {
        if lossemul == 0 {
            sip_tm.transmitData(self.userv, self.data, self.address, "" /*checksum*/, self.tid.CallId, 0 /*lossemul*/)
        } else {
            lossemul -= 1
        }
    }
    last_timeout *= 2
    rert_t := StartTimeout(func() { self.retrUasResponse(last_timeout, lossemul) }, self, last_timeout, 1, self.logger)
    self.prov_inflight_lock.Lock()
    self.prov_inflight.t = rert_t
    self.prov_inflight_lock.Unlock()
}

func (self *serverTransaction) SetPrackCBs(prack_cb func(sippy_types.SipRequest, sippy_types.SipResponse), noprack_cb func(*sippy_time.MonoTime)) {
    self.prack_cb = prack_cb
    self.noprack_cb = noprack_cb
}

func (self *serverTransaction) Setup100rel(req sippy_types.SipRequest) {
    for _, require := range req.GetSipRequire() {
        if require.HasTag("100rel") {
            self.pr_rel = true
            return
        }
    }
    for _, supported := range req.GetSipSupported() {
        if supported.HasTag("100rel") {
            self.pr_rel = true
            return
        }
    }
}

func (self *serverTransaction) PrRel() bool {
    return self.pr_rel
}

func (self *serverTransaction) UpdateUservFromUA(ua sippy_types.UA) {
    local_addr := ua.GetSourceAddress()
    if local_addr != nil {
        self.userv = self.sip_tm.l4r.getServer(local_addr, /*is_local*/ true)
    }
}
