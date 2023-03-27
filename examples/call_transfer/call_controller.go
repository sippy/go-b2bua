//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2019 Sippy Software, Inc. All rights reserved.
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

package main

import (
    "sync"

    "github.com/sippy/go-b2bua/sippy"
    //"github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/types"
)

var Next_cc_id chan int64

type callController struct {
    uaA             sippy_types.UA
    uaO             sippy_types.UA
    lock            *sync.Mutex // this must be a reference to prevent memory leak
    id              int64
    cmap            *CallMap
    evTry           *sippy.CCEventTry
    transfer_is_in_progress bool
}

func NewCallController(cmap *CallMap) *callController {
    self := &callController{
        id              : <-Next_cc_id,
        uaO             : nil,
        lock            : new(sync.Mutex),
        cmap            : cmap,
        transfer_is_in_progress : false,
    }
    self.uaA = sippy.NewUA(cmap.Sip_tm, cmap.config, cmap.config.Nh_addr, self, self.lock, nil)
    self.uaA.SetDeadCb(self.aDead)
    //self.uaA.SetCreditTime(5 * time.Second)
    return self
}

func (self *callController) handle_transfer(event sippy_types.CCEvent, ua sippy_types.UA) {
    switch ua {
    case self.uaA:
        if _, ok := event.(*sippy.CCEventConnect); ok {
            // Transfer is completed.
            self.transfer_is_in_progress = false
        }
        self.uaO.RecvEvent(event)
    case self.uaO:
        if _, ok := event.(*sippy.CCEventPreConnect); ok {
            //
            // Convert into CCEventUpdate.
            //
            // Here 200 OK response from the new callee has been received
            // and now re-INVITE will be sent to the caller.
            //
            // The CCEventPreConnect is here because the outgoing call to the
            // new destination has been sent using the late offer model, i.e.
            // the outgoing INVITE was body-less.
            //
            event = sippy.NewCCEventUpdate(event.GetRtime(), event.GetOrigin(), event.GetReason(),
                event.GetMaxForwards(), event.GetBody().GetCopy())
        }
        self.uaA.RecvEvent(event)
    }
}

func (self *callController) RecvEvent(event sippy_types.CCEvent, ua sippy_types.UA) {
    if self.transfer_is_in_progress {
        self.handle_transfer(event, ua)
        return
    }
    if ua == self.uaA {
        if self.uaO == nil {
            ev_try, ok := event.(*sippy.CCEventTry)
            if ! ok {
                // Some weird event received
                self.uaA.RecvEvent(sippy.NewCCEventDisconnect(nil, event.GetRtime(), ""))
                return
            }
            self.uaO = sippy.NewUA(self.cmap.Sip_tm, self.cmap.config, self.cmap.config.Nh_addr, self, self.lock, nil)
            self.uaO.SetRAddr(self.cmap.config.Nh_addr)
            self.evTry = ev_try
        }
        self.uaO.RecvEvent(event)
    } else {
        if ev_disc, ok := event.(*sippy.CCEventDisconnect); ok {
            redirect_url := ev_disc.GetRedirectURL()
            if redirect_url != nil {
                //
                // Either REFER or a BYE with Also: has been received from the callee.
                //
                // Do not interrupt the caller call leg and create a new call leg
                // to the new destination.
                //
                cld := redirect_url.GetUrl().Username

                //nh_addr := &sippy_net.HostPort{ redirect_url.GetUrl().Host, redirect_url.GetUrl().Port }
                nh_addr := self.cmap.config.Nh_addr

                self.uaO = sippy.NewUA(self.cmap.Sip_tm, self.cmap.config, nh_addr, self, self.lock, nil)
                ev_try, _ := sippy.NewCCEventTry(self.evTry.GetSipCallId(),
                    self.evTry.GetCLI(), cld, nil /*body*/, nil /*auth*/, self.evTry.GetCallerName(),
                    ev_disc.GetRtime(), self.evTry.GetOrigin())
                self.transfer_is_in_progress = true
                self.uaO.RecvEvent(ev_try)
                return
            }
        }
        self.uaA.RecvEvent(event)
    }
}

func (self *callController) aDead() {
    self.cmap.Remove(self.id)
}

func (self *callController) Shutdown() {
    self.uaA.Disconnect(nil, "")
}

func (self *callController) String() string {
    res := "uaA:" + self.uaA.String() + ", uaO: "
    if self.uaO == nil {
        res += "nil"
    } else {
        res += self.uaO.String()
    }
    return res
}

