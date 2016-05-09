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
    "sippy/time"
    "sippy/types"
)

type serverTransaction struct {
    baseTransaction
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
}

func NewServerTransaction(req sippy_types.SipRequest, checksum string, tid *sippy_header.TID, userv sippy_types.UdpServer, sip_tm *sipTransactionManager) sippy_types.ServerTransaction {
    needack := false
    var r487 sippy_types.SipResponse
    var branch string
    var expires time.Duration = 0
    method := req.GetMethod()
    if method == "INVITE" {
        needack = true
        r487 = req.GenResponse(487, "Request Terminated", /*body*/ nil, /*server*/ nil)
        branch = req.GetVias()[0].GetBranch()
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
    }
    self.baseTransaction = newBaseTransaction(self, tid, userv, sip_tm, nil, nil, needack)
    return self
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
}

func (self *serverTransaction) startTeE(t time.Duration) {
    self.teE = NewTimeout(self.timerE, self, t, 1, nil)
    self.teE.Start()
}

func (self *serverTransaction) startTeF(t time.Duration) {
    if self.teF != nil {
        self.teF.Cancel()
    }
    self.teF = NewTimeout(self.timerF, self, t, 1, nil)
    self.teF.Start()
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
    self.teD = NewTimeout(self.timerD, self, 32 * time.Second, 1, nil)
    self.teD.Start()
}

func (self *serverTransaction) timerD() {
    if self.sip_tm == nil {
        return
    }
    //print("timerD", t, t.GetTID())
    if self.noack_cb != nil && self.state != CONFIRMED {
        self.noack_cb(nil)
    }
    self.sip_tm.tserver_del(self.tid)
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
            self.r487.SetReason("Request Expired")
        }
        self.doCancel(/*rtime*/nil, /*req*/nil)
    }
}

// Timer to retransmit the last provisional reply every
// 2 seconds
func (self *serverTransaction) timerF() {
    if self.sip_tm == nil {
        return
    }
    //print("timerF", t.GetTID())
    self.cancelTeF()
    if self.state == RINGING && self.sip_tm.provisional_retr > 0 {
        self.sip_tm.transmitData(self.userv, self.data, self.address, /*checksum*/ "", self.tid.CallId)
        self.startTeF(time.Duration(self.sip_tm.provisional_retr))
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
    if self.sip_tm == nil {
        return
    }
    //println("existing transaction")
    if req.GetMethod() == self.method {
        // Duplicate received, check that we have sent any response on this
        // request already
        if self.data != nil && len(self.data) > 0 {
            self.sip_tm.transmitData(self.userv, self.data, self.address, checksum, self.tid.CallId)
        }
        return
    }
    if req.GetMethod() == "CANCEL" {
        // RFC3261 says that we have to reply 200 OK in all cases if
        // there is such transaction
        resp := req.GenResponse(200, "OK", /*body*/ nil, self.server)
        self.sip_tm.transmitMsg(self.userv, resp, resp.GetVias()[0].GetTAddr(self.sip_tm.config), checksum, self.tid.CallId)
        if self.state == TRYING || self.state == RINGING {
            self.doCancel(req.GetRtime(), req)
        }
    } else if req.GetMethod() == "ACK" && self.state == COMPLETED {
        self.state = CONFIRMED
        self.cancelTeA()
        self.cancelTeD()
        if self.ack_cb != nil {
            self.ack_cb(req)
        }
        // We have done with the transaction, no need to wait for timeout
        self.sip_tm.tserver_del(self.tid)
        self.sip_tm.rcache_put(checksum, &rcache_entry{
                                        userv : nil,
                                        data  : nil,
                                        address : nil,
                                        call_id : self.tid.CallId,
                                    })
        self.cleanup()
    }
}

func (self *serverTransaction) SendResponse(resp sippy_types.SipResponse, retrans bool, ack_cb func(sippy_types.SipRequest)) {
    if self.sip_tm == nil {
        return
    }
    if self.state != TRYING && self.state != RINGING && ! retrans {
        self.sip_tm.logError("BUG: attempt to send reply on already finished transaction!!!")
    }
    if resp.GetSCodeNum() > 100 && resp.GetTo().GetTag() == "" {
        resp.GetTo().GenTag()
    }
    self.data = []byte(resp.LocalStr(self.userv.GetLaddress(), /*compact*/ false))
    self.address = resp.GetVias()[0].GetTAddr(self.sip_tm.config)
    need_cleanup := false
    if resp.GetSCodeNum() < 200 {
        self.state = RINGING
        if self.sip_tm.provisional_retr > 0 && resp.GetSCodeNum() > 100 {
            self.startTeF(self.sip_tm.provisional_retr)
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
                    old_tid := *tid // copy
                    tid.Branch = ""
                    tid.ToTag = resp.GetTo().GetTag()
                    self.sip_tm.tserver_replace(&old_tid, tid, self)
                }
            }
            // Install retransmit timer if necessary
            self.tout = time.Duration(0.5 * float64(time.Second))
            self.startTeA()
        } else {
            // We have done with the transaction
            self.sip_tm.tserver_del(self.tid)
            need_cleanup = true
        }
    }
    self.sip_tm.transmitData(self.userv, self.data, self.address, self.checksum, self.tid.CallId)
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
