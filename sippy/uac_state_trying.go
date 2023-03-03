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
    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

type UacStateTrying struct {
    *uaStateGeneric
}

func NewUacStateTrying(ua sippy_types.UA, config sippy_conf.Config) *UacStateTrying {
    return &UacStateTrying{
        uaStateGeneric : newUaStateGeneric(ua, config),
    }
}

func (self *UacStateTrying) String() string {
    return "Trying(UAC)"
}

func (self *UacStateTrying) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) (sippy_types.UaState, func()) {
    var err error
    var event sippy_types.CCEvent

    body := resp.GetBody()
    code, reason := resp.GetSCode()
    self.ua.SetLastScode(code)

    if self.ua.HasNoReplyTimer() {
        self.ua.CancelNoReplyTimer()
        if code == 100 && self.ua.GetNpMtime() != nil {
            self.ua.StartNoProgressTimer()
        } else if code < 200 && self.ua.GetExMtime() != nil {
            self.ua.StartExpireTimer(resp.GetRtime())
        }
    }
    if code == 100 {
        self.ua.SetP100Ts(resp.GetRtime())
        self.ua.Enqueue(NewCCEventRing(code, reason, body, resp.GetRtime(), self.ua.GetOrigin()))
        return nil, nil
    }
    if self.ua.HasNoProgressTimer() {
        self.ua.CancelNoProgressTimer()
        if code < 200 && self.ua.GetExMtime() != nil {
            self.ua.StartExpireTimer(resp.GetRtime())
        }
    }
    if rseq := resp.GetRSeq(); rseq != nil {
        if ! tr.CheckRSeq(rseq) {
            // bad RSeq number - ignore the response
            return nil, nil
        }
        to_body, err := resp.GetTo().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #7: " + err.Error())
            return nil, nil
        }
        tag := to_body.GetTag()
        rUri, err := self.ua.GetRUri().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #8: " + err.Error())
            return nil, nil
        }
        rUri.SetTag(tag)
        cseq, err := resp.GetCSeq().GetBody()
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #9: " + err.Error())
            return nil, nil
        }
        req, err := self.ua.GenRequest("PRACK", nil, nil)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #10: " + err.Error())
            return nil, nil
        }
        rack := sippy_header.NewSipRAck(rseq.Number, cseq.CSeq, cseq.Method)
        req.AppendHeader(rack)
        self.ua.BeginNewClientTransaction(req, nil)
    }
    if code > 100 && code < 300 {
        // the route set must be ready for sending the PRACK
        self.ua.UpdateRouting(resp, true, true)
    }
    if code < 200 {
        event := NewCCEventRing(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, func(x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                self.ua.SetP1xxTs(resp.GetRtime())
                return NewUacStateRinging(self.ua, self.config), func() { self.ua.RingCb(resp.GetRtime(), self.ua.GetOrigin(), code) }
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        self.ua.SetP1xxTs(resp.GetRtime())
        return NewUacStateRinging(self.ua, self.config), func() { self.ua.RingCb(resp.GetRtime(), self.ua.GetOrigin(), code) }
    }
    self.ua.CancelExpireTimer()
    if code >= 200 && code < 300 {
        var to_body *sippy_header.SipAddress
        var rUri *sippy_header.SipAddress
        var cb func()

        to_body, err = resp.GetTo().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #1: " + err.Error())
            return nil, nil
        }
        tag := to_body.GetTag()
        if tag == "" {
            var req sippy_types.SipRequest

            //logger.Debug("tag-less 200 OK, disconnecting")
            self.ua.Enqueue(NewCCEventFail(502, "Bad Gateway", resp.GetRtime(), self.ua.GetOrigin()))
            // Generate and send BYE
            req, err = self.ua.GenRequest("BYE", nil, nil)
            if err != nil {
                self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #2: " + err.Error())
                return nil, nil
            }
            self.ua.BeginNewClientTransaction(req, nil)
            if self.ua.GetSetupTs() != nil && !self.ua.GetSetupTs().After(resp.GetRtime()) {
                self.ua.SetDisconnectTs(resp.GetRtime())
            } else {
                now, _ := sippy_time.NewMonoTime()
                self.ua.SetDisconnectTs(now)
            }
            return NewUaStateFailed(self.ua, self.config), func() { self.ua.FailCb(resp.GetRtime(), self.ua.GetOrigin(), code) }
        }
        rUri, err = self.ua.GetRUri().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #3: " + err.Error())
            return nil, nil
        }
        rUri.SetTag(tag)
        if !self.ua.GetLateMedia() || body == nil {
            self.ua.SetLateMedia(false)
            event = NewCCEventConnect(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
            self.ua.StartCreditTimer(resp.GetRtime())
            self.ua.SetConnectTs(resp.GetRtime())
            cb = func() { self.ua.ConnCb(resp.GetRtime(), self.ua.GetOrigin()) }
        } else {
            event = NewCCEventPreConnect(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
            tr.SetUAck(true)
            self.ua.SetPendingTr(tr)
        }
        newstate := NewUaStateConnected(self.ua, self.config)
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, func(x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                self.ua.SetConnectTs(resp.GetRtime())
                return newstate, cb
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        return newstate, cb
    }
    if (code == 301 || code == 302) && len(resp.GetContacts()) > 0 {
        var contact *sippy_header.SipAddress

        contact, err = resp.GetContacts()[0].GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #4: " + err.Error())
            return nil, nil
        }
        event = NewCCEventRedirect(code, reason, body,
                    []*sippy_header.SipAddress{ contact.GetCopy() },
                    resp.GetRtime(), self.ua.GetOrigin())
    } else if code == 300 && len(resp.GetContacts()) > 0 {
        urls := make([]*sippy_header.SipAddress, 0)
        for _, contact := range resp.GetContacts() {
            var cbody *sippy_header.SipAddress

            cbody, err = contact.GetBody(self.config)
            if err != nil {
                self.config.ErrorLogger().Error("UacStateTrying::RecvResponse: #5: " + err.Error())
                return nil, nil
            }
            urls = append(urls, cbody.GetCopy())
        }
        event = NewCCEventRedirect(code, reason, body, urls, resp.GetRtime(), self.ua.GetOrigin())
    } else {
        event_fail := NewCCEventFail(code, reason, resp.GetRtime(), self.ua.GetOrigin())
        event = event_fail
        if self.ua.GetPassAuth() {
            if code == 401 && len(resp.GetSipWWWAuthenticates()) > 0 {
                event_fail.challenges = make([]sippy_header.SipHeader, len(resp.GetSipWWWAuthenticates()))
                for i, hdr := range resp.GetSipWWWAuthenticates() {
                    event_fail.challenges[i] = hdr.GetCopy()
                }
            } else if code == 407 && len(resp.GetSipProxyAuthenticates()) > 0 {
                event_fail.challenges = make([]sippy_header.SipHeader, len(resp.GetSipProxyAuthenticates()))
                for i, hdr := range resp.GetSipProxyAuthenticates() {
                    event_fail.challenges[i] = hdr.GetCopy()
                }
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
    return NewUaStateFailed(self.ua, self.config), func() { self.ua.FailCb(resp.GetRtime(), self.ua.GetOrigin(), code) }
}

func (self *UacStateTrying) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
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
        return NewUacStateCancelling(self.ua, self.config), func() { self.ua.DiscCb(event.GetRtime(), event.GetOrigin(), self.ua.GetLastScode(), nil) }, nil
    }
    //return nil, fmt.Errorf("uac-trying: wrong event %s in the Trying state", event.String())
    return nil, nil, nil
}

func (self *UacStateTrying) ID() sippy_types.UaStateID {
    return sippy_types.UAC_STATE_TRYING
}
