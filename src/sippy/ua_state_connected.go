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
    "sippy/headers"
    "sippy/time"
    "sippy/types"
)

type UaStateConnected struct {
    uaStateGeneric
    ka_tr       sippy_types.ClientTransaction
    rtime       *sippy_time.MonoTime
    origin      string
}

func NewUaStateConnected(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string) *UaStateConnected {
    ua.SetBranch("")
    self := &UaStateConnected{
        uaStateGeneric : newUaStateGeneric(ua),
        ka_tr       : nil,
        rtime       : rtime,
        origin      : origin,
    }
    newKeepaliveController(ua)
    self.connected = true
    return self
}

func (self *UaStateConnected) OnActivation() {
    if self.rtime != nil {
        for _, listener := range self.ua.GetConnCbs() {
            listener(self.rtime, self.origin)
        }
    }
}

func (self *UaStateConnected) String() string {
    return "Connected"
}

func (self *UaStateConnected) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) sippy_types.UaState {
    if req.GetMethod() == "REFER" {
        if req.GetReferTo() == nil {
            t.SendResponse(req.GenResponse(400, "Bad Request", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
            return nil
        }
        t.SendResponse(req.GenResponse(202, "Accepted", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
        also := req.GetReferTo().GetUrl().GetCopy()
        self.ua.Enqueue(NewCCEventDisconnect(also, req.GetRtime(), self.ua.GetOrigin()))
        self.ua.RecvEvent(NewCCEventDisconnect(nil, req.GetRtime(), self.ua.GetOrigin()))
        return nil
    }
    if req.GetMethod() == "INVITE" {
        self.ua.SetUasResp(req.GenResponse(100, "Trying", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()))
        t.SendResponse(self.ua.GetUasResp(), false, nil)
        body := req.GetBody()
        if body == nil {
            // Some brain-damaged stacks use body-less re-INVITE as a means
            // for putting session on hold. Quick and dirty hack to make this
            // scenario working.
            body = self.ua.GetRSDP().GetCopy()
            body.GetParsedBody().SetCHeaderAddr("0.0.0.0")
        } else if self.ua.GetRSDP().String() == body.String() {
            t.SendResponse(req.GenResponse(200, "OK", self.ua.GetLSDP(), /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
            return nil
        }
        event := NewCCEventUpdate(/*rtime*/ req.GetRtime(), /*origin*/ self.ua.GetOrigin(), body)
        event.SetReason(req.GetReason())
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, req, func (x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                return NewUasStateUpdating(self.ua)
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        return NewUasStateUpdating(self.ua)
    }
    if req.GetMethod() == "BYE" {
        t.SendResponse(req.GenResponse(200, "OK", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
        //print "BYE received in the Connected state, going to the Disconnected state"
        var also *sippy_header.SipURL
        if len(req.GetAlso()) > 0 {
            also = req.GetAlso()[0].GetUrl().GetCopy()
        }
        event := NewCCEventDisconnect(also, req.GetRtime(), self.ua.GetOrigin())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(req.GetRtime())
        return NewUaStateDisconnected(self.ua, req.GetRtime(), self.ua.GetOrigin(), 0)
    }
    if req.GetMethod() == "INFO" {
        t.SendResponse(req.GenResponse(200, "OK", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
        event := NewCCEventInfo(/*rtime*/ req.GetRtime(), /*origin*/ self.ua.GetOrigin(), req.GetBody())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        return nil
    }
    if req.GetMethod() == "OPTIONS" || req.GetMethod() == "UPDATE" {
        t.SendResponse(req.GenResponse(200, "OK", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
        return nil
    }
    //print "wrong request %s in the state Connected" % req.GetMethod()
    return nil
}

func (self *UaStateConnected) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, error) {
    eh := event.GetExtraHeaders()
    ok := false
    var redirect *sippy_header.SipURL = nil

    switch ev := event.(type) {
    case *CCEventDisconnect:
        redirect = ev.GetRedirectURL()
        ok = true
    case *CCEventRedirect:
        redirect = ev.GetRedirectURL()
        ok = true
    case *CCEventFail:
        ok = true
    }
    if ok {
        //print "event", event, "received in the Connected state sending BYE"
        if redirect != nil && self.ua.ShouldUseRefer() {
            req := self.ua.GenRequest("REFER", nil, "", "", nil, eh...)
            self.ua.IncLCSeq()
            also := sippy_header.NewSipReferTo(/*address*/ sippy_header.NewSipAddress("", /*url*/ redirect))
            req.AppendHeader(also)
            rby := sippy_header.NewSipReferredBy(/*address*/ sippy_header.NewSipAddress("", /*url*/ self.ua.GetLUri().GetUrl()))
            req.AppendHeader(rby)
            self.ua.SipTM().NewClientTransaction(req, newRedirectController(self.ua), self.ua.GetSessionLock(), /*laddress*/ self.ua.GetSourceAddress(), nil)
        } else {
            req := self.ua.GenRequest("BYE", nil, "", "", nil, eh...)
            self.ua.IncLCSeq()
            if redirect != nil {
                also := sippy_header.NewSipAlso(/*address*/ sippy_header.NewSipAddress("", /*url*/ redirect))
                req.AppendHeader(also)
            }
            self.ua.SipTM().NewClientTransaction(req, nil, self.ua.GetSessionLock(), /*laddress*/ self.ua.GetSourceAddress(), nil)
        }
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateDisconnected(self.ua, event.GetRtime(), event.GetOrigin(), 0), nil
    }
    if _event, ok := event.(*CCEventUpdate); ok {
        body := _event.GetBody()
        if self.ua.GetLSDP().String() == body.String() {
            if self.ua.GetRSDP() != nil {
                self.ua.Enqueue(NewCCEventConnect(200, "OK", self.ua.GetRSDP().GetCopy(), /*rtime*/ event.GetRtime(), /*origin*/ event.GetOrigin()))
            } else {
                self.ua.Enqueue(NewCCEventConnect(200, "OK", nil, /*rtime*/ event.GetRtime(), /*origin*/ event.GetOrigin()))
            }
            return nil, nil
        }
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            self.ua.OnLocalSdpChange(body, event, func() { self.ua.RecvEvent(event) })
            return nil, nil
        }
        req := self.ua.GenRequest("INVITE", body, "", "", nil, eh...)
        self.ua.IncLCSeq()
        self.ua.SetLSDP(body)
        t, err := self.ua.SipTM().NewClientTransaction(req, self.ua, self.ua.GetSessionLock(), /*laddress*/ self.ua.GetSourceAddress(), nil)
        if err != nil {
            return nil, err
        }
        t.SetOutboundProxy(self.ua.GetOutboundProxy())
        self.ua.SetClientTransaction(t)
        return NewUacStateUpdating(self.ua), nil
    }
    if _event, ok := event.(*CCEventInfo); ok {
        body := _event.GetBody()
        req := self.ua.GenRequest("INFO", nil, "", "", nil, eh...)
        req.SetBody(body)
        self.ua.IncLCSeq()
        self.ua.SipTM().NewClientTransaction(req, nil, self.ua.GetSessionLock(), /*laddress*/ self.ua.GetSourceAddress(), nil)
        return nil, nil
    }
    if _event, ok := event.(*CCEventConnect); ok && self.ua.GetPendingTr() != nil {
    //if self.ua.GetPendingTr() != nil && isinstance(event, CCEventConnect) {
        self.ua.CancelExpireTimer()
        //code, reason, body = event.getData()
        body := _event.GetBody()
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            self.ua.OnLocalSdpChange(body, event, func() { self.ua.RecvEvent(event) })
            return nil, nil
        }
        self.ua.StartCreditTimer(event.GetRtime())
        self.ua.SetConnectTs(event.GetRtime())
        self.ua.SetLSDP(body)
        self.ua.GetPendingTr().GetACK().SetBody(body)
        self.ua.GetPendingTr().SendACK()
        self.ua.SetPendingTr(nil)
        for _, listener := range self.ua.GetConnCbs() {
            listener(event.GetRtime(), self.ua.GetOrigin())
        }
    }
    //print "wrong event %s in the Connected state" % event
    return nil, nil
}

func (self *UaStateConnected) OnStateChange() {
    if self.ka_tr != nil {
        self.ka_tr.Cancel()
        self.ka_tr = nil
    }
    if self.ua.GetPendingTr() != nil {
        self.ua.GetPendingTr().SendACK()
        self.ua.SetPendingTr(nil)
    }
    self.ua.CancelExpireTimer()
}

func (self *UaStateConnected) RecvACK(req sippy_types.SipRequest) {
    body := req.GetBody()
    //scode = ('ACK', 'ACK', body)
    event := NewCCEventConnect(0, "ACK", nil, /*rtime*/ req.GetRtime(), /*origin*/ self.ua.GetOrigin())
    self.ua.CancelExpireTimer()
    self.ua.StartCreditTimer(req.GetRtime())
    self.ua.SetConnectTs(req.GetRtime())
    for _, listener := range self.ua.GetConnCbs() {
        listener(req.GetRtime(), self.ua.GetOrigin())
    }
    if body != nil {
        if self.ua.HasOnRemoteSdpChange() {
            self.ua.OnRemoteSdpChange(body, req, func (x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
            return
        } else {
            self.ua.SetRSDP(body.GetCopy())
        }
    } else {
        self.ua.SetRSDP(nil)
    }
    self.ua.Enqueue(event)
    return
}
