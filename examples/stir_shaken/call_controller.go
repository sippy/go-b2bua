//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2021 Sippy Software, Inc. All rights reserved.
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
    "time"

    "sippy"
    "sippy/headers"
    "sippy/types"
)

type callController struct {
    uaA             sippy_types.UA
    uaO             sippy_types.UA
    lock            *sync.Mutex // this must be a reference to prevent memory leak
    id              int64
    cmap            *callMap
    identity_hf     sippy_header.SipHeader
    date_hf         *sippy_header.SipDate
}

func NewCallController(cmap *callMap, identity_hf sippy_header.SipHeader, date_hf *sippy_header.SipDate) *callController {
    self := &callController{
        id              : <-next_cc_id,
        uaO             : nil,
        lock            : new(sync.Mutex),
        cmap            : cmap,
        identity_hf     : identity_hf,
        date_hf         : date_hf,
    }
    self.uaA = sippy.NewUA(cmap.sip_tm, cmap.config, cmap.config.nh_addr, self, self.lock, nil)
    self.uaA.SetDeadCb(self.aDead)
    //self.uaA.SetCreditTime(5 * time.Second)
    return self
}

func (self *callController) RecvEvent(event sippy_types.CCEvent, ua sippy_types.UA) {
    if ua == self.uaA {
        if ev_try, ok := event.(*sippy.CCEventTry); ok {
            if false && ! self.SshakenVerify(ev_try) {
                self.uaA.RecvEvent(sippy.NewCCEventFail(608, "Rejected", event.GetRtime(), ""))
                return
            }
        }
        if self.uaO == nil {
            ev_try, ok := event.(*sippy.CCEventTry)
            if ! ok {
                self.uaA.RecvEvent(sippy.NewCCEventDisconnect(nil, event.GetRtime(), ""))
                return
            }
            self.uaO = sippy.NewUA(self.cmap.sip_tm, self.cmap.config, self.cmap.config.nh_addr, self, self.lock, nil)
            identity, date, err := self.SshakenAuth(ev_try.GetCLI(), ev_try.GetCLD())
            if err == nil {
                extra_headers := []sippy_header.SipHeader{
                    sippy_header.NewSipDate(date),
                    sippy_header.NewSipGenericHF("Identity", identity),
                }
                self.uaO.SetExtraHeaders(extra_headers)
            }
            self.uaO.SetDeadCb(self.oDead)
            self.uaO.SetRAddr(self.cmap.config.nh_addr)
        }
        self.uaO.RecvEvent(event)
    } else {
        self.uaA.RecvEvent(event)
    }
}

func (self *callController) SshakenVerify(ev_try *sippy.CCEventTry) bool {
    if self.identity_hf == nil || self.date_hf == nil {
        return false
    }
    identity := self.identity_hf.StringBody()
    date_ts, err := self.date_hf.GetTime()
    if err != nil {
        self.cmap.logger.Error("Error parsing Date: header: " + err.Error())
        return false
    }
    orig_tn := ev_try.GetCLI()
    dest_tn := ev_try.GetCLD()
    err = self.cmap.sshaken.Verify(identity, orig_tn, dest_tn, date_ts)
    return err == nil
}

func (self *callController) SshakenAuth(cli, cld string) (string, time.Time, error) {
    date_ts := time.Now()
    identity, err := self.cmap.sshaken.Authenticate(date_ts, cli, cld)
    return identity, date_ts, err
}

func (self *callController) aDead() {
    self.cmap.Remove(self.id)
}

func (self *callController) oDead() {
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
