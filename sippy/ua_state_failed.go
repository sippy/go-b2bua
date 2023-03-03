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
    "github.com/sippy/go-b2bua/sippy/types"
)

type UaStateFailed struct {
    *uaStateGeneric
}

func NewUaStateFailed(ua sippy_types.UA, config sippy_conf.Config) *UaStateFailed {
    self := &UaStateFailed{
        uaStateGeneric  : newUaStateGeneric(ua, config),
    }
    ua.ResetOnLocalSdpChange()
    ua.ResetOnRemoteSdpChange()
    return self
}

func (self *UaStateFailed) OnActivation() {
    StartTimeout(self.goDead, self.ua.GetSessionLock(), self.ua.GetGoDeadTimeout(), 1, self.config.ErrorLogger())
}

func (self *UaStateFailed) String() string {
    return "Failed"
}

func (self *UaStateFailed) goDead() {
    //print 'Time in Failed state expired, going to the Dead state'
    self.ua.ChangeState(NewUaStateDead(self.ua, self.config), nil)
}

func (self *UaStateFailed) ID() sippy_types.UaStateID {
    return sippy_types.UA_STATE_FAILED
}
