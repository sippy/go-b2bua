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
    "sippy/types"
    "sippy/time"
    "sippy/headers"
)

type UasStateRinging struct {
    uaStateGeneric
    rtime   *sippy_time.MonoTime
    origin  string
    scode   int
}

func NewUasStateRinging(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string, scode int) *UasStateRinging {
    return &UasStateRinging{
        uaStateGeneric  : newUaStateGeneric(ua),
        rtime           : rtime,
        origin          : origin,
        scode           : scode,
    }
}

func (self *UasStateRinging) OnActivation() {
    if self.rtime != nil {
        for _, listener := range self.ua.GetRingCbs() {
            listener(self.rtime, self.origin, self.scode)
        }
    }
}

func (self *UasStateRinging) String() string {
    return "Ringing(UAS)"
}

func (self *UasStateRinging) RecvEvent(_event sippy_types.CCEvent) (sippy_types.UaState, error) {
    eh := _event.GetExtraHeaders()
    switch event :=_event.(type) {
    case *CCEventRing:
        code, reason, body := event.scode, event.scode_reason, event.body
        if code == 0 {
            code, reason, body = 180, "Ringing", nil
        } else {
            if code == 100 {
                return nil, nil
            }
            if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
                self.ua.OnLocalSdpChange(body, event, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
                return nil, nil
            }
        }
        self.ua.SetLSDP(body)
        if self.ua.GetP1xxTs() == nil {
            self.ua.SetP1xxTs(event.GetRtime())
        }
        self.ua.SendUasResponse(nil, code, reason, body, nil, false, eh...)
        for _, ring_cb := range self.ua.GetRingCbs() {
            ring_cb(event.rtime, event.origin, code)
        }
        return nil, nil
    case *CCEventConnect:
        code, reason, body := event.scode, event.scode_reason, event.body
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            self.ua.OnLocalSdpChange(body, event, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil
        }
        self.ua.SetLSDP(body)
        self.ua.SendUasResponse(nil, code, reason, body, self.ua.GetLContact(), false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.StartCreditTimer(event.GetRtime())
        self.ua.SetConnectTs(event.GetRtime())
        return NewUaStateConnected(self.ua, event.GetRtime(), event.GetOrigin()), nil
    case *CCEventPreConnect:
        code, reason, body := event.scode, event.scode_reason, event.body
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            self.ua.OnLocalSdpChange(body, event, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil
        }
        self.ua.SetLSDP(body)
        self.ua.SendUasResponse(nil, code, reason, body, self.ua.GetLContact(), /*ack_wait*/ true, eh...)
        return NewUaStateConnected(self.ua, nil, ""), nil
    case *CCEventRedirect:
        code, reason, body, redirect_url := event.scode, event.scode_reason, event.body, event.redirect_url
        if code == 0 {
            code, reason, body, redirect_url = 500, "Failed", nil, nil
        }
        contact := sippy_header.NewSipContactFromAddress(sippy_header.NewSipAddress("", redirect_url))
        self.ua.SendUasResponse(nil, code, reason, body, contact, false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateFailed(self.ua, event.GetRtime(), event.GetOrigin(), code), nil
    case *CCEventFail:
        code, reason := event.scode, event.scode_reason
        if code == 0 {
            code, reason = 500, "Failed"
        }
        self.ua.SendUasResponse(nil, code, reason, nil, nil, false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateFailed(self.ua, event.GetRtime(), event.GetOrigin(), code), nil
    case *CCEventDisconnect:
        self.ua.SendUasResponse(nil, 500, "Disconnected", nil, nil, false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateDisconnected(self.ua, event.GetRtime(), event.GetOrigin(), self.ua.GetLastScode()), nil
    }
    //return nil, fmt.Errorf("wrong event %s in the Ringing state", _event.String())
    return nil, nil
}

func (self *UasStateRinging) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) sippy_types.UaState {
    if req.GetMethod() == "BYE" {
        self.ua.SendUasResponse(t, 487, "Request Terminated", nil, nil, false)
        t.SendResponse(req.GenResponse(200, "OK", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        //print "BYE received in the Ringing state, going to the Disconnected state"
        var also *sippy_header.SipURL = nil
        if len(req.GetAlso()) > 0 {
            also = req.GetAlso()[0].GetUrl().GetCopy()
        }
        event := NewCCEventDisconnect(also, req.GetRtime(), self.ua.GetOrigin())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        self.ua.CancelExpireTimer()
        self.ua.SetDisconnectTs(req.GetRtime())
        return NewUaStateDisconnected(self.ua, req.GetRtime(), self.ua.GetOrigin(), 0)
    }
    return nil
}

func (self *UasStateRinging) Cancel(rtime *sippy_time.MonoTime, req sippy_types.SipRequest) {
    event := NewCCEventDisconnect(nil, rtime, self.ua.GetOrigin())
    if req != nil {
        event.SetReason(req.GetReason())
    }
    self.ua.SetDisconnectTs(rtime)
    self.ua.ChangeState(NewUaStateDisconnected(self.ua, rtime, self.ua.GetOrigin(), 0))
    self.ua.EmitEvent(event)
}
