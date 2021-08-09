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
    "sippy/conf"
    "sippy/time"
    "sippy/types"
)

type UasStateUpdating struct {
    *uaStateGeneric
}

func NewUasStateUpdating(ua sippy_types.UA, config sippy_conf.Config) *UasStateUpdating {
    self := &UasStateUpdating{
        uaStateGeneric : newUaStateGeneric(ua, config),
    }
    self.connected = true
    return self
}

func (self *UasStateUpdating) String() string {
    return "Updating(UAS)"
}

func (self *UasStateUpdating) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) (sippy_types.UaState, func()) {
    if req.GetMethod() == "INVITE" {
        t.SendResponseWithLossEmul(req.GenResponse(491, "Request Pending", nil, self.ua.GetLocalUA().AsSipServer()), false, nil, self.ua.UasLossEmul())
        return nil, nil
    } else if req.GetMethod() == "BYE" {
        self.ua.SendUasResponse(t, 487, "Request Terminated", nil, nil, false)
        t.SendResponseWithLossEmul(req.GenResponse(200, "OK", nil, self.ua.GetLocalUA().AsSipServer()), false, nil, self.ua.UasLossEmul())
        //print "BYE received in the Updating state, going to the Disconnected state"
        event := NewCCEventDisconnect(nil, req.GetRtime(), self.ua.GetOrigin())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(req.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(req.GetRtime(), self.ua.GetOrigin(), 0, req) }
    } else if req.GetMethod() == "REFER" {
        if req.GetReferTo() == nil {
            t.SendResponseWithLossEmul(req.GenResponse(400, "Bad Request", nil, self.ua.GetLocalUA().AsSipServer()), false, nil, self.ua.UasLossEmul())
            return nil, nil
        }
        self.ua.SendUasResponse(t, 487, "Request Terminated", nil, nil, false)
        t.SendResponseWithLossEmul(req.GenResponse(202, "Accepted", nil, self.ua.GetLocalUA().AsSipServer()), false, nil, self.ua.UasLossEmul())
        refer_to, err := req.GetReferTo().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UasStateUpdating::RecvRequest: #1: " + err.Error())
            return nil, nil
        }
        self.ua.Enqueue(NewCCEventDisconnect(refer_to.GetCopy(), req.GetRtime(), self.ua.GetOrigin()))
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(req.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(req.GetRtime(), self.ua.GetOrigin(), 0, req) }
    }
    //print "wrong request %s in the state Updating" % req.getMethod()
    return nil, nil
}

func (self *UasStateUpdating) RecvEvent(_event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    eh := _event.GetExtraHeaders()
    switch event := _event.(type) {
    case *CCEventRing:
        code, reason, body := event.scode, event.scode_reason, event.body
        if code == 0 {
            code, reason, body = 180, "Ringing", nil
        }
        if body != nil && body.NeedsUpdate() && self.ua.HasOnLocalSdpChange() {
            self.ua.OnLocalSdpChange(body, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil, nil
        }
        self.ua.SetLSDP(body)
        self.ua.SendUasResponse(nil, code, reason, body, nil, false, eh...)
        return nil, nil, nil
    case *CCEventPreConnect:
        code, reason, body := event.scode, event.scode_reason, event.body
        if body != nil && body.NeedsUpdate() && self.ua.HasOnLocalSdpChange() {
            self.ua.OnLocalSdpChange(body, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil, nil
        }
        self.ua.SetLSDP(body)
        self.ua.SendUasResponse(nil, code, reason, body, self.ua.GetLContacts(), true /*ack_wait*/, eh...)
        return NewUasStatePreConnect(self.ua, self.config, true /*confirm_connect*/), nil, nil
    case *CCEventConnect:
        code, reason, body := event.scode, event.scode_reason, event.body
        if body != nil && body.NeedsUpdate() && self.ua.HasOnLocalSdpChange() {
            self.ua.OnLocalSdpChange(body, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil, nil
        }
        self.ua.SetLSDP(body)
        self.ua.SendUasResponse(nil, code, reason, body, self.ua.GetLContacts(), true, eh...)
        return NewUasStatePreConnect(self.ua, self.config, false /*confirm_connect*/), nil, nil
    case *CCEventRedirect:
        self.ua.SendUasResponse(nil, event.scode, event.scode_reason, event.body, event.GetContacts(), false, eh...)
        return NewUaStateConnected(self.ua, self.config), nil, nil
    case *CCEventFail:
        code, reason := event.scode, event.scode_reason
        if code == 0 {
            code, reason = 500, "Failed"
        }
        if event.warning != nil {
            eh = append(eh, event.warning)
        }
        self.ua.SendUasResponse(nil, code, reason, nil, nil, false, eh...)
        return NewUaStateConnected(self.ua, self.config), nil, nil
    case *CCEventDisconnect:
        self.ua.SendUasResponse(nil, 487, "Request Terminated", nil, nil, false, eh...)
        req, err := self.ua.GenRequest("BYE", nil, nil, eh...)
        if err != nil {
            return nil, nil, err
        }
        self.ua.BeginNewClientTransaction(req, nil)
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(event.GetRtime(), event.GetOrigin(), 0, nil) }, nil
    }
    //return nil, fmt.Errorf("wrong event %s in the Updating state", _event.String())
    return nil, nil, nil
}

func (self *UasStateUpdating) RecvCancel(rtime *sippy_time.MonoTime, inreq sippy_types.SipRequest) {
    req, err := self.ua.GenRequest("BYE", nil, nil)
    if err != nil {
        self.config.ErrorLogger().Error("UasStateUpdating::Cancel: #1: " + err.Error())
        return
    }
    self.ua.BeginNewClientTransaction(req, nil)
    self.ua.CancelCreditTimer()
    self.ua.SetDisconnectTs(rtime)
    self.ua.ChangeState(NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(rtime, self.ua.GetOrigin(), 0, inreq) })
    event := NewCCEventDisconnect(nil, rtime, self.ua.GetOrigin())
    if inreq != nil {
        event.SetReason(inreq.GetReason())
    }
    self.ua.EmitEvent(event)
}

func (self *UasStateUpdating) ID() sippy_types.UaStateID {
    return sippy_types.UAS_STATE_UPDATING
}
