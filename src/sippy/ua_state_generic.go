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
    "sippy/time"
    "sippy/types"
)

type uaStateGeneric struct {
    ua          sippy_types.UA
    connected   bool
    config      sippy_conf.Config
}

func newUaStateGeneric(ua sippy_types.UA, config sippy_conf.Config) *uaStateGeneric {
    return &uaStateGeneric{
        ua          : ua,
        connected   : false,
        config      : config,
    }
}

func (self *uaStateGeneric) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) (sippy_types.UaState, func()) {
    return nil, nil
}

func (self *uaStateGeneric) RecvResponse(resp sippy_types.SipResponse, t sippy_types.ClientTransaction) (sippy_types.UaState, func()) {
    return nil, nil
}

func (self *uaStateGeneric) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    return nil, nil, nil
}

func (self *uaStateGeneric) RecvCancel(rtime *sippy_time.MonoTime, req sippy_types.SipRequest) {
}

func (*uaStateGeneric) OnDeactivate() {
}

func (*uaStateGeneric) OnActivation() {
}

func (*uaStateGeneric) RecvACK(sippy_types.SipRequest) {
}

func (self *uaStateGeneric) IsConnected() bool {
    return self.connected
}

func (self *uaStateGeneric) RecvPRACK(req sippy_types.SipRequest, resp sippy_types.SipResponse) {
}
