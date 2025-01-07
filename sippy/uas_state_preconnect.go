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
    "github.com/sippy/go-b2bua/sippy/types"
)

type UasStatePreConnect struct {
    *uaStateGeneric
    pending_ev_update   *CCEventUpdate
    confirm_connect     bool
}

func NewUasStatePreConnect(ua sippy_types.UA, config sippy_conf.Config, confirm_connect bool) *UasStatePreConnect {
    ua.SetBranch("")
    self := &UasStatePreConnect{
        uaStateGeneric  : newUaStateGeneric(ua, config),
        confirm_connect : confirm_connect,
    }
    self.connected = true
    return self
}

func (self *UasStatePreConnect) String() string {
    return "PreConnect(UAS)"
}

func (self *UasStatePreConnect) try_other_events(event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    var redirect *sippy_header.SipAddress = nil
    switch ev := event.(type) {
    case *CCEventDisconnect:
        redirect = ev.GetRedirectURL()
    case *CCEventRedirect:
        redirect = ev.GetRedirectURL()
    case *CCEventFail:
    default:
        //fmt.Printf("wrong event %s in the %s state", event.String(), self.ID().String())
        return nil, nil, nil
    }
    //println("event", event.String(), "received in the Connected state sending BYE")
    eh := event.GetExtraHeaders()
    if redirect != nil && self.ua.ShouldUseRefer() {
        var lUri *sippy_header.SipAddress

        req, err := self.ua.GenRequest("REFER", nil, nil, eh...)
        if err != nil {
            return nil, nil, err
        }
        also := sippy_header.NewSipReferTo(redirect)
        req.AppendHeader(also)
        lUri, err = self.ua.GetLUri().GetBody(self.config)
        if err != nil {
            return nil, nil, err
        }
        rby := sippy_header.NewSipReferredBy(sippy_header.NewSipAddress("", lUri.GetUrl()))
        req.AppendHeader(rby)
        self.ua.BeginNewClientTransaction(req, newRedirectController(self.ua))
    } else {
        req, err := self.ua.GenRequest("BYE", nil, nil, eh...)
        if err != nil {
            return nil, nil, err
        }
        if redirect != nil {
            also := sippy_header.NewSipAlso(redirect)
            req.AppendHeader(also)
        }
        self.ua.BeginNewClientTransaction(req, nil)
    }
    self.ua.CancelCreditTimer()
    self.ua.SetDisconnectTs(event.GetRtime())
    return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(event.GetRtime(), event.GetOrigin(), 0, nil) }, nil
}

func (self *UasStatePreConnect) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) (sippy_types.UaState, func()) {
    if req.GetMethod() == "INVITE" {
        t.SendResponseWithLossEmul(req.GenResponse(491, "Request Pending", nil, self.ua.GetLocalUA().AsSipServer()), false, nil, self.ua.UasLossEmul())
        return nil, nil
    } else if req.GetMethod() == "BYE" {
        self.ua.SendUasResponse(t, 200, "OK", nil, nil, false)
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
            self.config.ErrorLogger().Error("UasStatePreConnect::RecvRequest: #1: " + err.Error())
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

func (self *UasStatePreConnect) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    switch ev := event.(type) {
    case *CCEventUpdate:
        self.pending_ev_update = ev
        return nil, nil, nil
    case *CCEventInfo:
        body := ev.GetBody()
        req, err := self.ua.GenRequest("INFO", nil, nil, event.GetExtraHeaders()...)
        if err != nil {
            return nil, nil, err
        }
        req.SetBody(body)
        self.ua.BeginNewClientTransaction(req, nil)
        return nil, nil, nil
    default:
        return self.try_other_events(event)
    }
}

func (self *UasStatePreConnect) OnDeactivate() {
    self.ua.CancelExpireTimer()
}

func (self *UasStatePreConnect) RecvACK(req sippy_types.SipRequest) {
    var event *CCEventConnect
    var cb func()
    rtime := req.GetRtime()
    origin := self.ua.GetOrigin()
    if self.confirm_connect {
        body := req.GetBody()
        event = NewCCEventConnect(0, "ACK", body, rtime, origin)
        self.ua.CancelExpireTimer()
        self.ua.CancelCreditTimer() // prevent timer leak
        self.ua.StartCreditTimer(rtime)
        self.ua.SetConnectTs(rtime)
        self.ua.ConnCb(rtime, origin)
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                ev := event
                event = nil // do not send this event via EmitEvent below
                self.ua.OnRemoteSdpChange(body, func (x sippy_types.MsgBody, ex sippy_types.SipHandlingError) { self.ua.DelayedRemoteSdpUpdate(ev, x, ex) })
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        cb = func() { self.ua.ConnCb(rtime, origin) }
    }
    self.ua.ChangeState(NewUaStateConnected(self.ua, self.config), cb)
    if event != nil {
        self.ua.EmitEvent(event)
    }
    if self.pending_ev_update != nil {
        self.ua.RecvEvent(self.pending_ev_update)
        self.pending_ev_update = nil
    }
}

func (self *UasStatePreConnect) ID() sippy_types.UaStateID {
    return sippy_types.UAS_STATE_PRE_CONNECT
}
