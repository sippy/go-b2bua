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

type UacStateCancelling struct {
    *uaStateGeneric
    te      *Timeout
}

func NewUacStateCancelling(ua sippy_types.UA, config sippy_conf.Config) *UacStateCancelling {
    self := &UacStateCancelling{
        uaStateGeneric  : newUaStateGeneric(ua, config),
    }
    ua.ResetOnLocalSdpChange()
    ua.ResetOnRemoteSdpChange()
    // 300 provides good estimate on the amount of time during which
    // we can wait for receiving non-negative response to CANCELled
    // INVITE transaction.
    return self
}

func (self *UacStateCancelling) OnActivation() {
    self.te = StartTimeout(self.goIdle, self.ua.GetSessionLock(), 300.0, 1, self.config.ErrorLogger())
}

func (self *UacStateCancelling) String() string {
    return "Cancelling(UAC)"
}

func (self *UacStateCancelling) goIdle() {
    //print "Time in Cancelling state expired, going to the Dead state"
    self.te = nil
    self.ua.ChangeState(NewUaStateDead(self.ua, self.config), nil)
}

func (self *UacStateCancelling) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) (sippy_types.UaState, func()) {
    code, _ := resp.GetSCode()
    if code < 200 {
        return nil, nil
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
        var err error
        var rUri *sippy_header.SipAddress
        var to_body *sippy_header.SipAddress
        var req sippy_types.SipRequest

        self.ua.UpdateRouting(resp, true, true)
        rUri, err = self.ua.GetRUri().GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateCancelling::RecvResponse: #1: " + err.Error())
            return nil, nil
        }
        to_body, err = resp.GetTo().GetBody(self.config)
        rUri.SetTag(to_body.GetTag())
        req, err = self.ua.GenRequest("BYE", nil, nil)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateCancelling::RecvResponse: #2: " + err.Error())
            return nil, nil
        }
        self.ua.SipTM().BeginNewClientTransaction(req, nil, self.ua.GetSessionLock(), /*laddress*/ self.ua.GetSourceAddress(), nil, self.ua.BeforeRequestSent)
        return NewUaStateDisconnected(self.ua, self.config), nil
    }
    return NewUaStateDead(self.ua, self.config), nil
}

func (self *UacStateCancelling) ID() sippy_types.UaStateID {
    return sippy_types.UAC_STATE_CANCELLING
}
