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

type UaStateDisconnected struct {
    *uaStateGeneric
    rtime   *sippy_time.MonoTime
    origin  string
    scode   int
    inreq   sippy_types.SipRequest
}

func NewUaStateDisconnected(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string, scode int, inreq sippy_types.SipRequest) *UaStateDisconnected {
    self := &UaStateDisconnected{
        uaStateGeneric  : newUaStateGeneric(ua),
        rtime           : rtime,
        origin          : origin,
        scode           : scode,
        inreq           : inreq,
    }
    ua.ResetOnLocalSdpChange()
    ua.ResetOnRemoteSdpChange()
    return self
}

func (self *UaStateDisconnected) OnActivation() {
    if self.rtime != nil {
        for _, listener := range self.ua.GetDiscCbs() {
            listener(self.rtime, self.origin, self.scode, self.inreq)
        }
    }
    StartTimeout(self.goDead, self.ua.GetSessionLock(), self.ua.GetGoDeadTimeout(), 1, self.ua.Config().ErrorLogger())
}

func (self *UaStateDisconnected) String() string {
    return "Disconnected"
}

func (self *UaStateDisconnected) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) sippy_types.UaState {
    if req.GetMethod() == "BYE" {
        //print "BYE received in the Disconnected state"
        t.SendResponse(req.GenResponse(200, "OK", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
    } else {
        t.SendResponse(req.GenResponse(500, "Disconnected", nil, /*server*/ self.ua.GetLocalUA().AsSipServer()), false, nil)
    }
    return nil
}

func (self *UaStateDisconnected) goDead() {
    //print "Time in Disconnected state expired, going to the Dead state"
    self.ua.ChangeState(NewUaStateDead(self.ua, nil, ""))
}
