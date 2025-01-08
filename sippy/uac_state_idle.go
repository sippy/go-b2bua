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
    "time"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

type UacStateIdle struct {
    *uaStateGeneric
    config sippy_conf.Config
}

func NewUacStateIdle(ua sippy_types.UA, config sippy_conf.Config) *UacStateIdle {
    return &UacStateIdle{
        uaStateGeneric  : newUaStateGeneric(ua, config),
        config          : config,
    }
}

func (self *UacStateIdle) String() string {
    return "Idle(UAC)"
}

func (self *UacStateIdle) RecvEvent(_event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    var err error
    var rUri *sippy_header.SipAddress
    var lUri *sippy_header.SipAddress
    var contact *sippy_header.SipAddress
    var req sippy_types.SipRequest
    var tr sippy_types.ClientTransaction

    switch event := _event.(type) {
    case *CCEventTry:
        if self.ua.GetSetupTs() == nil {
            self.ua.SetSetupTs(event.rtime)
        }
        self.ua.SetOrigin("callee")
        body := event.GetBody()
        if body != nil {
            if body.NeedsUpdate() && self.ua.HasOnLocalSdpChange() {
                self.ua.OnLocalSdpChange(body, self.ua.GetDelayedLocalSdpUpdate(event))
                return nil, nil, nil
            }
        } else {
            self.ua.SetLateMedia(true)
        }
        if event.GetSipCallId() == nil {
            self.ua.SetCallId(sippy_header.GenerateSipCallId(self.config))
        } else {
            self.ua.SetCallId(event.GetSipCallId().GetCopy())
        }
        self.ua.SetRTarget(sippy_header.NewSipURL(event.GetCLD(), self.ua.GetRAddr0().Host, self.ua.GetRAddr0().Port, false))
        self.ua.SetRUri(sippy_header.NewSipTo(sippy_header.NewSipAddress("", self.ua.GetRTarget().GetCopy()), self.config))
        rUri, err = self.ua.GetRUri().GetBody(self.config)
        if err != nil {
            return nil, nil, err
        }
        rUri.GetUrl().Port = nil
        self.ua.SetLUri(sippy_header.NewSipFrom(sippy_header.NewSipAddress(event.GetCallerName(), sippy_header.NewSipURL(event.GetCLI(), self.config.GetMyAddress(), self.config.GetMyPort(), false)), self.config))
        self.ua.RegConsumer(self.ua, self.ua.GetCallId().CallId)
        lUri, err = self.ua.GetLUri().GetBody(self.config)
        if err != nil {
            return nil, nil, err
        }
        lUri.GetUrl().Port = nil
        lUri.SetTag(self.ua.GetLTag())
        self.ua.SetLCSeq(200)
        if self.ua.GetLContact() == nil {
            if src_addr := self.ua.GetSourceAddress(); src_addr != nil {
                self.ua.SetLContact(sippy_header.NewSipContactFromHostPort(src_addr.Host, src_addr.Port))
            } else {
                self.ua.SetLContact(sippy_header.NewSipContact(self.config))
            }
        }
        contact, err = self.ua.GetLContact().GetBody(self.config)
        if err != nil {
            return nil, nil, err
        }
        contact.GetUrl().Username = event.GetCLI()
        self.ua.SetRoutes(event.routes)
        self.ua.SetLSDP(body)
        eh := event.GetExtraHeaders()
        if event.GetMaxForwards() != nil {
            eh = append(eh, event.GetMaxForwards())
        }
        self.ua.OnUacSetupComplete()
        req, err = self.ua.GenRequest("INVITE", body, /*Challenge*/ nil, eh...)
        if err != nil {
            return nil, nil, err
        }
        tr, err = self.ua.PrepTr(req, eh)
        if err != nil {
            return nil, nil, err
        }
        self.ua.SetClientTransaction(tr)
        self.ua.BeginClientTransaction(req, tr)
        if self.ua.PassAuth() && event.GetSipAuthorizationHF() != nil {
            req.AppendHeader(event.GetSipAuthorizationHF())
        }
        self.ua.SetAuth(nil)

        if self.ua.GetExpireTime() > 0 {
            self.ua.SetExMtime(event.GetRtime().Add(self.ua.GetExpireTime()))
        }
        if self.ua.GetNoProgressTime() > 0 && (self.ua.GetExpireTime() <= 0 || self.ua.GetNoProgressTime() < self.ua.GetExpireTime()) {
            self.ua.SetNpMtime(event.GetRtime().Add(self.ua.GetNoProgressTime()))
        }
        if (self.ua.GetNoReplyTime() > 0 && self.ua.GetNoReplyTime() < time.Duration(32 * time.Second)) &&
          (self.ua.GetExpireTime() <= 0 || self.ua.GetNoReplyTime() < self.ua.GetExpireTime()) &&
          (self.ua.GetNoProgressTime() <= 0 || self.ua.GetNoReplyTime() < self.ua.GetNoProgressTime()) {
            self.ua.SetNrMtime(event.GetRtime().Add(self.ua.GetNoReplyTime()))
        }
        if self.ua.GetNrMtime() != nil {
            self.ua.StartNoReplyTimer()
        } else if self.ua.GetNpMtime() != nil {
            self.ua.StartNoProgressTimer()
        } else if self.ua.GetExMtime() != nil {
            self.ua.StartExpireTimer(event.GetRtime())
        }
        return NewUacStateTrying(self.ua, self.config), nil, nil
    case *CCEventFail:
    case *CCEventRedirect:
    case *CCEventDisconnect:
    default:
        return nil, nil, nil
    }
    if self.ua.GetSetupTs() != nil && ! _event.GetRtime().Before(self.ua.GetSetupTs()) {
        self.ua.SetDisconnectTs(_event.GetRtime())
    } else {
        disconnect_ts, _ := sippy_time.NewMonoTime()
        self.ua.SetDisconnectTs(disconnect_ts)
    }
    return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(_event.GetRtime(), _event.GetOrigin(), 0, nil) }, nil
}

func (self *UacStateIdle) ID() sippy_types.UaStateID {
    return sippy_types.UAC_STATE_IDLE
}
