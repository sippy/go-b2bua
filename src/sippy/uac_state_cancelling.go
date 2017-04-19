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
    "sippy/time"
    "sippy/types"
)

type UacStateCancelling struct {
    *uaStateGeneric
    te      *Timeout
    rtime   *sippy_time.MonoTime
    origin  string
    scode   int
}

func NewUacStateCancelling(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string, scode int) *UacStateCancelling {
    self := &UacStateCancelling{
        uaStateGeneric  : newUaStateGeneric(ua),
        rtime           : rtime,
        origin          : origin,
        scode           : scode,
    }
    ua.ResetOnLocalSdpChange()
    ua.ResetOnRemoteSdpChange()
    // 300 provides good estimate on the amount of time during which
    // we can wait for receiving non-negative response to CANCELled
    // INVITE transaction.
    self.te = StartTimeout(self.goIdle, self.ua.GetSessionLock(), 300.0, 1, self.ua.Config().ErrorLogger())
    return self
}

func (self *UacStateCancelling) OnActivation() {
    if self.rtime != nil {
        self.ua.DiscCb(self.rtime, self.origin, self.scode, nil)
    }
}

func (self *UacStateCancelling) String() string {
    return "Cancelling(UAC)"
}

func (self *UacStateCancelling) goIdle() {
    //print "Time in Cancelling state expired, going to the Dead state"
    self.te = nil
    self.ua.ChangeState(NewUaStateDead(self.ua, nil, ""))
}

func (self *UacStateCancelling) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) sippy_types.UaState {
    code, _ := resp.GetSCode()
    if code < 200 {
        return nil
    }
    if self.te != nil {
        self.te.Cancel()
        self.te = nil
    }
    // When the final response arrives make sure to send BYE
    // if response is positive 200 OK and move into
    // UaStateDisconnected to catch any in-flight BYE from the
    // called party.
    //
    // If the response is negative or redirect go to the UaStateDead
    // immediately, since this means that we won"t receive any more
    // requests from the calling party. XXX: redirects should probably
    // somehow reported to the upper level, but it will create
    // significant additional complexity there, since after signalling
    // Failure/Disconnect calling party don"t expect any more
    // events to be delivered from the called one. In any case,
    // this should be fine, since we are in this state only when
    // caller already has declared his wilingless to end the session,
    // so that he is probably isn"t interested in redirects anymore.
    if code >= 200 && code < 300 {
        self.ua.UpdateRouting(resp, true, true)
        self.ua.GetRUri().SetTag(resp.GetTo().GetTag())
        req := self.ua.GenRequest("BYE", nil, "", "", nil)
        self.ua.IncLCSeq()
        self.ua.SipTM().NewClientTransaction(req, nil, self.ua.GetSessionLock(), /*laddress*/ self.ua.GetSourceAddress(), nil, self.ua.BeforeRequestSent)
        return NewUaStateDisconnected(self.ua, nil, "", 0, nil)
    }
    return NewUaStateDead(self.ua, nil, "")
}

func (self *UacStateCancelling) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, error) {
    //return nil, fmt.Errorf("wrong event %s in the Cancelling state", event.String())
    return nil, nil
}
