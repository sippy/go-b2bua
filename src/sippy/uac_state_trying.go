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
)

type UacStateTrying struct {
    *uaStateGeneric
}

func NewUacStateTrying(ua sippy_types.UA) *UacStateTrying {
    return &UacStateTrying{
        uaStateGeneric : newUaStateGeneric(ua),
    }
}

func (self *UacStateTrying) OnActivation() {
}

func (self *UacStateTrying) String() string {
    return "Trying(UAC)"
}

func (self *UacStateTrying) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) sippy_types.UaState {
    body := resp.GetBody()
    code, reason := resp.GetSCode()
    self.ua.SetLastScode(code)

    if self.ua.HasNoReplyTimer() {
        self.ua.CancelNoReplyTimer()
        if code == 100 && self.ua.GetNpMtime() != nil {
            self.ua.StartNoProgressTimer(self.ua.GetNpMtime())
        } else if code < 200 && self.ua.GetExMtime() != nil {
            self.ua.StartExpireTimer(self.ua.GetExMtime())
        }
    }
    if code == 100 {
        self.ua.SetP100Ts(resp.GetRtime())
        self.ua.Enqueue(NewCCEventRing(code, reason, body, resp.GetRtime(), self.ua.GetOrigin()))
        return nil
    }
    if self.ua.HasNoProgressTimer() {
        self.ua.CancelNoProgressTimer()
        if code < 200 && self.ua.GetExMtime() != nil {
            self.ua.StartExpireTimer(self.ua.GetExMtime())
        }
    }
    if code < 200 {
        event := NewCCEventRing(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, resp, func(x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                self.ua.SetP1xxTs(resp.GetRtime())
                return NewUacStateRinging(self.ua, resp.GetRtime(), self.ua.GetOrigin(), code)
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        self.ua.SetP1xxTs(resp.GetRtime())
        return NewUacStateRinging(self.ua, resp.GetRtime(), self.ua.GetOrigin(), code)
    }
    self.ua.CancelExpireTimer()
    if code >= 200 && code < 300 {
        self.ua.UpdateRouting(resp, true, true)
        tag := resp.GetTo().GetTag()
        if tag == "" {
            //logger.Debug("tag-less 200 OK, disconnecting")
            self.ua.Enqueue(NewCCEventFail(502, "Bad Gateway", resp.GetRtime(), self.ua.GetOrigin()))
            // Generate and send BYE
            req := self.ua.GenRequest("BYE", nil, "", "", nil)
            self.ua.IncLCSeq()
            self.ua.SipTM().NewClientTransaction(req, nil, self.ua.GetSessionLock(), self.ua.GetSourceAddress(), nil, self.ua.BeforeRequestSent)
            if self.ua.GetSetupTs() != nil && !self.ua.GetSetupTs().After(resp.GetRtime()) {
                self.ua.SetDisconnectTs(resp.GetRtime())
            } else {
                now, _ := sippy_time.NewMonoTime()
                self.ua.SetDisconnectTs(now)
            }
            return NewUaStateFailed(self.ua, resp.GetRtime(), self.ua.GetOrigin(), code)
        }
        self.ua.GetRUri().SetTag(tag)
        var rval sippy_types.UaState
        var event sippy_types.CCEvent
        if !self.ua.GetLateMedia() || body == nil {
            self.ua.SetLateMedia(false)
            event = NewCCEventConnect(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
            self.ua.StartCreditTimer(resp.GetRtime())
            self.ua.SetConnectTs(resp.GetRtime())
            rval = NewUaStateConnected(self.ua, resp.GetRtime(), self.ua.GetOrigin())
        } else {
            event = NewCCEventPreConnect(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
            tr.SetUAck(true)
            self.ua.SetPendingTr(tr)
            rval = NewUaStateConnected(self.ua, nil, "")
        }
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, resp, func(x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                self.ua.SetConnectTs(resp.GetRtime())
                return rval
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        return rval
    }
    var event sippy_types.CCEvent
    if (code == 301 || code == 302) && len(resp.GetContacts()) > 0 {
        event = NewCCEventRedirect(code, reason, body, resp.GetContacts()[0].GetUrl().GetCopy(), resp.GetRtime(), self.ua.GetOrigin())
    } else {
        event_fail := NewCCEventFail(code, reason, resp.GetRtime(), self.ua.GetOrigin())
        event = event_fail
        if self.ua.GetPassAuth() {
            if code == 401 && resp.GetSipWWWAuthenticate() != nil {
                event_fail.challenge = resp.GetSipWWWAuthenticate().GetCopy()
            } else if code == 407 && resp.GetSipProxyAuthenticate() != nil {
                event_fail.challenge = resp.GetSipProxyAuthenticate().GetCopy()
            }
        }
        if resp.GetReason() != nil {
            event_fail.sip_reason = resp.GetReason().GetCopy()
        }
        event.SetReason(resp.GetReason())
    }
    self.ua.Enqueue(event)
    if self.ua.GetSetupTs() != nil && !self.ua.GetSetupTs().After(resp.GetRtime()) {
        self.ua.SetDisconnectTs(resp.GetRtime())
    } else {
        now, _ := sippy_time.NewMonoTime()
        self.ua.SetDisconnectTs(now)
    }
    return NewUaStateFailed(self.ua, resp.GetRtime(), self.ua.GetOrigin(), code)
}

func (self *UacStateTrying) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, error) {
    cancel_transaction := false
    switch event.(type) {
    case *CCEventFail: cancel_transaction = true
    case *CCEventRedirect: cancel_transaction = true
    case *CCEventDisconnect: cancel_transaction = true
    }
    if cancel_transaction {
        self.ua.GetClientTransaction().Cancel(event.GetExtraHeaders()...)
        self.ua.CancelExpireTimer()
        self.ua.CancelNoProgressTimer()
        self.ua.CancelNoReplyTimer()
        if self.ua.GetSetupTs() != nil && !self.ua.GetSetupTs().After(event.GetRtime()) {
            self.ua.SetDisconnectTs(event.GetRtime())
        } else {
            now, _ := sippy_time.NewMonoTime()
            self.ua.SetDisconnectTs(now)
        }
        return NewUacStateCancelling(self.ua, event.GetRtime(), event.GetOrigin(), self.ua.GetLastScode()), nil
    }
    //return nil, fmt.Errorf("uac-trying: wrong event %s in the Trying state", event.String())
    return nil, nil
}
