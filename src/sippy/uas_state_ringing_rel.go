// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2020 Sippy Software, Inc. All rights reserved.
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
    "sippy/types"
)

type UasStateRingingRel struct {
    *UasStateRinging
    prack_received      bool
    prack_wait          bool
    pending_ev_ring     sippy_types.CCEvent
    pending_ev_connect  sippy_types.CCEvent
}

func NewUasStateRingingRel(ua sippy_types.UA, config sippy_conf.Config) *UasStateRingingRel {
    self := &UasStateRingingRel{
        UasStateRinging : NewUasStateRinging(ua, config),
        prack_wait      : false,
    }
    if ua.GetLSDP() != nil {
        self.prack_wait = true
    }
    return self
}

func (self *UasStateRingingRel) RecvEvent(_event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    switch event := _event.(type) {
    case *CCEventRing:
        if ! self.prack_received {
            // There is no PRACK for the previous response yet.
            if event.scode > 100 {
                // Memorize the last event
                self.pending_ev_ring = _event
            }
            return nil, nil, nil
        } else {
            self.prack_wait = event.body != nil
            self.prack_received = false
        }
    case *CCEventConnect:
        if self.prack_wait && ! self.prack_received {
            // 200 OK received but the last reliable provisional
            // response has not yet been aknowledged. Memorize the event
            // until PRACK is received.
            self.pending_ev_connect = _event
            return nil, nil, nil
        }
    }
    return self.UasStateRinging.RecvEvent(_event)
}

func (self *UasStateRingingRel) RecvPRACK(req sippy_types.SipRequest) {
    var state sippy_types.UaState
    var cb func()
    var err error

    self.prack_received = true
    if self.pending_ev_connect != nil {
        state, cb, err = self.RecvEvent(self.pending_ev_connect)
    } else if self.pending_ev_ring != nil {
        state, cb, err = self.RecvEvent(self.pending_ev_ring)
    }
    if err != nil {
        self.config.ErrorLogger().Error("RecvPRACK: " + err.Error())
    }
    if state != nil {
        self.ua.ChangeState(state, cb)
    }
    self.pending_ev_ring = nil
    self.pending_ev_connect = nil
}
