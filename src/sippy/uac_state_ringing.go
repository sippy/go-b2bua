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
    "sippy/headers"
    "sippy/types"
    "sippy/time"
)

type UacStateRinging struct {
    *uaStateGeneric
    triedauth   bool
    rtime       *sippy_time.MonoTime
    origin      string
    scode       int
}

func NewUacStateRinging(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string, scode int, config sippy_conf.Config) sippy_types.UaState {
    return &UacStateRinging{
        uaStateGeneric  : newUaStateGeneric(ua, config),
        triedauth       : false,
        rtime           : rtime,
        origin          : origin,
        scode           : scode,
    }
}

func (self *UacStateRinging) OnActivation() {
    if self.rtime != nil {
        self.ua.RingCb(self.rtime, self.origin, self.scode)
    }
}

func (self *UacStateRinging) String() string {
    return "Ringing(UAC)"
}

func (self *UacStateRinging) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) sippy_types.UaState {
    var err error
    var event sippy_types.CCEvent

    body := resp.GetBody()
    code, reason := resp.GetSCode()
    if code > 180 {
        // the 100 Trying can be processed later than 180 Ringing
        self.ua.SetLastScode(code)
    }
    if code < 200 {
        if self.ua.GetP1xxTs() == nil {
            self.ua.SetP1xxTs(resp.GetRtime())
        }
        event := NewCCEventRing(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
        self.ua.RingCb(resp.GetRtime(), self.ua.GetOrigin(), code)
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, resp, func(x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                return nil
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        return nil
    }
    self.ua.CancelExpireTimer()
    if code >= 200 && code < 300 {
        var to_body *sippy_header.SipAddress
        var rUri *sippy_header.SipAddress
        var rval sippy_types.UaState

        self.ua.UpdateRouting(resp, true, true)
        to_body, err = resp.GetTo().GetBody()
        if err != nil {
            self.config.ErrorLogger().Error("UacStateRinging::RecvResponse: #1: " + err.Error())
            return nil
        }
        tag := to_body.GetTag()
        if tag == "" {
            var req sippy_types.SipRequest

            //print "tag-less 200 OK, disconnecting"
            event := NewCCEventFail(502, "Bad Gateway", resp.GetRtime(), self.ua.GetOrigin())
            self.ua.Enqueue(event)
            req, err = self.ua.GenRequest("BYE", nil, "", "", nil)
            if err != nil {
                self.config.ErrorLogger().Error("UacStateRinging::RecvResponse: #2: " + err.Error())
                return nil
            }
            self.ua.IncLCSeq()
            self.ua.SipTM().BeginNewClientTransaction(req, nil, self.ua.GetSessionLock(), self.ua.GetSourceAddress(), nil, self.ua.BeforeRequestSent)
            if self.ua.GetSetupTs() != nil && !self.ua.GetSetupTs().After(resp.GetRtime())  {
                self.ua.SetDisconnectTs(resp.GetRtime())
            } else {
                now, _ := sippy_time.NewMonoTime()
                self.ua.SetDisconnectTs(now)
            }
            return NewUaStateFailed(self.ua, resp.GetRtime(), self.ua.GetOrigin(), 502, self.config)
        }
        rUri, err = self.ua.GetRUri().GetBody()
        if err != nil {
            self.config.ErrorLogger().Error("UacStateRinging::RecvResponse: #3: " + err.Error())
            return nil
        }
        rUri.SetTag(tag)
        if !self.ua.GetLateMedia() || body == nil {
            self.ua.SetLateMedia(false)
            event = NewCCEventConnect(code, reason, resp.GetBody(), resp.GetRtime(), self.ua.GetOrigin())
            self.ua.StartCreditTimer(resp.GetRtime())
            self.ua.SetConnectTs(resp.GetRtime())
            rval = NewUaStateConnected(self.ua, resp.GetRtime(), self.ua.GetOrigin(), self.config)
        } else {
            event = NewCCEventPreConnect(code, reason, resp.GetBody(), resp.GetRtime(), self.ua.GetOrigin())
            tr.SetUAck(true)
            self.ua.SetPendingTr(tr)
            rval = NewUaStateConnected(self.ua, nil, "", self.config)
        }
        self.ua.StartCreditTimer(resp.GetRtime())
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, resp, func (x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
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
    if (code == 301 || code == 302) && len(resp.GetContacts()) > 0 {
        var contact *sippy_header.SipAddress

        contact, err = resp.GetContacts()[0].GetBody()
        if err != nil {
            self.config.ErrorLogger().Error("UacStateRinging::RecvResponse: #4: " + err.Error())
            return nil
        }
        event = NewCCEventRedirect(code, reason, body,
                    []*sippy_header.SipAddress{ contact.GetCopy() },
                    resp.GetRtime(), self.ua.GetOrigin(), self.config)
    } else if code == 300 && len(resp.GetContacts()) > 0 {
        urls := make([]*sippy_header.SipAddress, 0)
        for _, contact := range resp.GetContacts() {
            var cbody *sippy_header.SipAddress

            cbody, err = contact.GetBody()
            if err != nil {
                self.config.ErrorLogger().Error("UacStateRinging::RecvResponse: #5: " + err.Error())
                return nil
            }
            urls = append(urls, cbody.GetCopy())
        }
        event = NewCCEventRedirect(code, reason, body, urls, resp.GetRtime(), self.ua.GetOrigin(), self.config)
    } else {
        event = NewCCEventFail(code, reason, resp.GetRtime(), self.ua.GetOrigin())
        event.SetReason(resp.GetReason())
    }
    self.ua.Enqueue(event)
    if self.ua.GetSetupTs() != nil && !self.ua.GetSetupTs().After(resp.GetRtime()) {
        self.ua.SetDisconnectTs(resp.GetRtime())
    } else {
        now, _ := sippy_time.NewMonoTime()
        self.ua.SetDisconnectTs(now)
    }
    return NewUaStateFailed(self.ua, resp.GetRtime(), self.ua.GetOrigin(), code, self.config)
}

func (self *UacStateRinging) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, error) {
    switch event.(type) {
    case *CCEventFail:
    case *CCEventRedirect:
    case *CCEventDisconnect:
    default:
        //return nil, fmt.Errorf("wrong event %s in the Ringing state", event.String())
        return nil, nil
    }
    self.ua.GetClientTransaction().Cancel()
    self.ua.CancelExpireTimer()
    if self.ua.GetSetupTs() != nil && ! self.ua.GetSetupTs().After(event.GetRtime()) {
        self.ua.SetDisconnectTs(event.GetRtime())
    } else {
        now, _ := sippy_time.NewMonoTime()
        self.ua.SetDisconnectTs(now)
    }
    return NewUacStateCancelling(self.ua, event.GetRtime(), event.GetOrigin(), self.ua.GetLastScode(), self.config), nil
}
