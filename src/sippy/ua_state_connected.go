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
)

type UaStateConnected struct {
    *uaStateGeneric
    ka_controller *keepaliveController
}

func NewUaStateConnected(ua sippy_types.UA, config sippy_conf.Config) *UaStateConnected {
    ua.SetBranch("")
    self := &UaStateConnected{
        uaStateGeneric : newUaStateGeneric(ua, config),
        ka_controller : newKeepaliveController(ua, config.ErrorLogger()),
    }
    self.connected = true
    return self
}

func (self *UaStateConnected) OnActivation() {
    if self.ka_controller != nil {
        self.ka_controller.Start()
    }
}

func (self *UaStateConnected) String() string {
    return "Connected"
}

func (self *UaStateConnected) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) (sippy_types.UaState, func()) {
    if req.GetMethod() == "REFER" {
        if req.GetReferTo() == nil {
            t.SendResponse(req.GenResponse(400, "Bad Request", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
            return nil, nil
        }
        t.SendResponse(req.GenResponse(202, "Accepted", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        refer_to, err := req.GetReferTo().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UaStateConnected::RecvRequest: #1: " + err.Error())
            return nil, nil
        }
        self.ua.Enqueue(NewCCEventDisconnect(refer_to.GetCopy(), req.GetRtime(), self.ua.GetOrigin()))
        self.ua.RecvEvent(NewCCEventDisconnect(nil, req.GetRtime(), self.ua.GetOrigin()))
        return nil, nil
    }
    if req.GetMethod() == "INVITE" {
        self.ua.SetUasResp(req.GenResponse(100, "Trying", nil, self.ua.GetLocalUA().AsSipServer()))
        t.SendResponse(self.ua.GetUasResp(), false, nil)
        body := req.GetBody()
        rsdp := self.ua.GetRSDP()
        if body != nil && rsdp != nil && rsdp.String() == body.String() {
            self.ua.SendUasResponse(t, 200, "OK", self.ua.GetLSDP(), self.ua.GetLContacts(), false /*ack_wait*/)
            return nil, nil
        }
        event := NewCCEventUpdate(req.GetRtime(), self.ua.GetOrigin(), req.GetReason(), req.GetMaxForwards(), body)
        self.ua.OnReinvite(req, event)
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                self.ua.OnRemoteSdpChange(body, func (x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) })
                return NewUasStateUpdating(self.ua, self.config), nil
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        return NewUasStateUpdating(self.ua, self.config), nil
    }
    if req.GetMethod() == "BYE" {
        t.SendResponse(req.GenResponse(200, "OK", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        //print "BYE received in the Connected state, going to the Disconnected state"
        var also *sippy_header.SipAddress
        if len(req.GetAlso()) > 0 {
            also_body, err := req.GetAlso()[0].GetBody(self.config)
            if err != nil {
                self.config.ErrorLogger().Error("UaStateConnected::RecvRequest: #3: " + err.Error())
                return nil, nil
            }
            also = also_body.GetCopy()
        }
        event := NewCCEventDisconnect(also, req.GetRtime(), self.ua.GetOrigin())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(req.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(req.GetRtime(), self.ua.GetOrigin(), 0, req) }
    }
    if req.GetMethod() == "INFO" {
        t.SendResponse(req.GenResponse(200, "OK", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        event := NewCCEventInfo(req.GetRtime(), self.ua.GetOrigin(), req.GetBody())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        return nil, nil
    }
    if req.GetMethod() == "OPTIONS" || req.GetMethod() == "UPDATE" {
        t.SendResponse(req.GenResponse(200, "OK", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        return nil, nil
    }
    //print "wrong request %s in the state Connected" % req.GetMethod()
    return nil, nil
}

func (self *UaStateConnected) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    var err error
    var req sippy_types.SipRequest

    eh := event.GetExtraHeaders()
    ok := false
    var redirect *sippy_header.SipAddress = nil

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
        //println("event", event.String(), "received in the Connected state sending BYE")
        if redirect != nil && self.ua.ShouldUseRefer() {
            var lUri *sippy_header.SipAddress

            req, err = self.ua.GenRequest("REFER", nil, nil, eh...)
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
            req, err = self.ua.GenRequest("BYE", nil, nil, eh...)
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
    if _event, ok := event.(*CCEventUpdate); ok {
        var tr sippy_types.ClientTransaction

        body := _event.GetBody()
        if self.ua.GetLSDP() != nil && body != nil && self.ua.GetLSDP().String() == body.String() {
            if self.ua.GetRSDP() != nil {
                self.ua.Enqueue(NewCCEventConnect(200, "OK", self.ua.GetRSDP().GetCopy(), event.GetRtime(), event.GetOrigin()))
            } else {
                self.ua.Enqueue(NewCCEventConnect(200, "OK", nil, event.GetRtime(), event.GetOrigin()))
            }
            return nil, nil, nil
        }
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            err := self.ua.OnLocalSdpChange(body, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            if err != nil {
                ev := NewCCEventFail(400, "Malformed SDP Body", event.GetRtime(), "")
                ev.SetWarning(err.Error())
                self.ua.Enqueue(ev)
            }
            return nil, nil, nil
        }
        if body == nil {
            self.ua.SetLateMedia(true)
        }
        eh2 := eh
        if _event.GetMaxForwards() != nil {
            var max_forwards *sippy_header.SipNumericHF

            max_forwards, err = _event.GetMaxForwards().GetBody()
            if err != nil {
                return nil, nil, err
            }
            if max_forwards.Number <= 0 {
                self.ua.Enqueue(NewCCEventFail(483, "Too Many Hops", event.GetRtime(), ""))
                return nil, nil, nil
            }
            eh2 = append(eh2, sippy_header.NewSipMaxForwards(max_forwards.Number - 1))
        }
        req, err = self.ua.GenRequest("INVITE", body, nil, eh2...)
        if err != nil {
            return nil, nil, err
        }
        self.ua.SetLSDP(body)
        tr, err = self.ua.PrepTr(req, nil)
        if err != nil {
            return nil, nil, err
        }
        self.ua.SetClientTransaction(tr)
        self.ua.BeginClientTransaction(req, tr)
        return NewUacStateUpdating(self.ua, self.config), nil, nil
    }
    if _event, ok := event.(*CCEventInfo); ok {
        body := _event.GetBody()
        req, err = self.ua.GenRequest("INFO", nil, nil, eh...)
        if err != nil {
            return nil, nil, err
        }
        req.SetBody(body)
        self.ua.BeginNewClientTransaction(req, nil)
        return nil, nil, nil
    }
    if _event, ok := event.(*CCEventConnect); ok && self.ua.GetPendingTr() != nil {
        self.ua.CancelExpireTimer()
        body := _event.GetBody()
        if body != nil && self.ua.HasOnLocalSdpChange() && body.NeedsUpdate() {
            self.ua.OnLocalSdpChange(body, func(sippy_types.MsgBody) { self.ua.RecvEvent(event) })
            return nil, nil, nil
        }
        self.ua.CancelCreditTimer() // prevent timer leak
        self.ua.StartCreditTimer(event.GetRtime())
        self.ua.SetConnectTs(event.GetRtime())
        self.ua.SetLSDP(body)
        self.ua.GetPendingTr().GetACK().SetBody(body)
        self.ua.GetPendingTr().SendACK()
        self.ua.SetPendingTr(nil)
        self.ua.ConnCb(event.GetRtime(), self.ua.GetOrigin())
    }
    //print "wrong event %s in the Connected state" % event
    return nil, nil, nil
}

func (self *UaStateConnected) OnDeactivate() {
    if self.ka_controller != nil {
        self.ka_controller.Stop()
    }
    if self.ua.GetPendingTr() != nil {
        self.ua.GetPendingTr().SendACK()
        self.ua.SetPendingTr(nil)
    }
    self.ua.CancelExpireTimer()
}

func (self *UaStateConnected) ID() sippy_types.UaStateID {
    return sippy_types.UA_STATE_CONNECTED
}
