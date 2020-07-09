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

type UasStateTrying struct {
    *uaStateGeneric
}

func NewUasStateTrying(ua sippy_types.UA, config sippy_conf.Config) *UasStateTrying {
    return &UasStateTrying{
        uaStateGeneric : newUaStateGeneric(ua, config),
    }
}

func (self *UasStateTrying) String() string {
    return "Trying(UAS)"
}

func (self *UasStateTrying) RecvEvent(_event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    eh := _event.GetExtraHeaders()
    switch event := _event.(type) {
    case *CCEventRing:
        code, reason, body := event.scode, event.scode_reason, event.body
        if code == 0 {
            code, reason, body = 180, "Ringing", nil
        } else {
            if code == 100 {
                return nil, nil, nil
            }
            if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
                self.ua.OnLocalSdpChange(body, func (sippy_types.MsgBody) { self.ua.RecvEvent(event) })
                return nil, nil, nil
            }
        }
        self.ua.SetLSDP(body)
        self.ua.SendUasResponse(nil, code, reason, body, self.ua.GetLContacts(), false, eh...)
        if self.ua.HasNoProgressTimer() {
            self.ua.CancelNoProgressTimer()
            if self.ua.GetExMtime() != nil {
                self.ua.StartExpireTimer(event.GetRtime())
            }
        }
        if self.ua.GetP1xxTs() == nil {
            self.ua.SetP1xxTs(event.GetRtime())
        }
        if self.ua.PrRel() {
            return NewUasStateRingingRel(self.ua, self.config), func() { self.ua.RingCb(event.GetRtime(), event.GetOrigin(), code) }, nil
        }
        return NewUasStateRinging(self.ua, self.config), func() { self.ua.RingCb(event.GetRtime(), event.GetOrigin(), code) }, nil
    case *CCEventConnect:
        var cb func()
        code, reason, body := event.scode, event.scode_reason, event.body
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            self.ua.OnLocalSdpChange(body, func (sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil, nil
        }
        self.ua.SetLSDP(body)
        self.ua.CancelNoProgressTimer()
        if ! event.GetLateMedia() {
            self.ua.CancelExpireTimer()
            self.ua.StartCreditTimer(event.GetRtime())
            self.ua.SetConnectTs(event.GetRtime())
            cb = func() { self.ua.ConnCb(event.GetRtime(), event.GetOrigin()) }
        }
        self.ua.SendUasResponse(nil, code, reason, body, self.ua.GetLContacts(), /*ack_wait*/ true, eh...)
        return NewUaStateConnected(self.ua, self.config), cb, nil
    case *CCEventRedirect:
        self.ua.SendUasResponse(nil, event.scode, event.scode_reason, event.body, event.GetContacts(), false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.CancelNoProgressTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateFailed(self.ua, self.config), func() { self.ua.FailCb(event.GetRtime(), event.GetOrigin(), event.scode) }, nil
    case *CCEventFail:
        code, reason := event.scode, event.scode_reason
        if code == 0 {
            code, reason = 500, "Failed"
        }
        self.ua.SendUasResponse(nil, code, reason, nil, nil, false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.CancelNoProgressTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateFailed(self.ua, self.config), func() { self.ua.FailCb(event.GetRtime(), event.GetOrigin(), code) }, nil
    case *CCEventDisconnect:
        code, reason := self.ua.OnEarlyUasDisconnect(event)
        eh = event.GetExtraHeaders()
        self.ua.SendUasResponse(nil, code, reason, nil, nil, false, eh...)
        self.ua.CancelExpireTimer()
        self.ua.CancelNoProgressTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(event.GetRtime(), event.GetOrigin(), self.ua.GetLastScode(), nil) }, nil
    }
    //return nil, fmt.Errorf("uas-trying: wrong event %s in the Trying state", _event.String())
    return nil, nil, nil
}

func (self *UasStateTrying) Cancel(rtime *sippy_time.MonoTime, req sippy_types.SipRequest) {
    event := NewCCEventDisconnect(nil, rtime, self.ua.GetOrigin())
    if req != nil {
        event.SetReason(req.GetReason())
    }
    self.ua.SetDisconnectTs(rtime)
    self.ua.ChangeState(NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(rtime, self.ua.GetOrigin(), 0, req) })
    self.ua.EmitEvent(event)
}

func (self *UasStateTrying) ID() sippy_types.UaStateID {
    return sippy_types.UAS_STATE_TRYING
}
