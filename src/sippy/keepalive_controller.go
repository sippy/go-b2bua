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
    "sippy/headers"
    "sippy/types"
)

type keepaliveController struct {
    ua          sippy_types.UA
    triedauth   bool
    ka_tr       sippy_types.ClientTransaction
    keepalives  int
}

func newKeepaliveController(ua sippy_types.UA) *keepaliveController {
    if ua.GetKaInterval() <= 0 {
        return nil
    }
    self := &keepaliveController{
        ua          : ua,
        triedauth   : false,
        keepalives  : 0,
    }
    StartTimeout(self.keepAlive, self.ua.GetSessionLock(), self.ua.GetKaInterval(), 1, self.ua.Config().ErrorLogger())
    return self
}

func (self *keepaliveController) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) {
    var err error
    if _, ok := self.ua.GetState().(*UaStateConnected); ! ok {
        return
    }
    code, _ := resp.GetSCode()
    if code == 401 && resp.GetSipWWWAuthenticate() != nil && self.ua.GetUsername() != "" && self.ua.GetPassword() != "" && ! self.triedauth {
        challenge := resp.GetSipWWWAuthenticate().GetBody()
        req := self.ua.GenRequest("INVITE", self.ua.GetLSDP(), challenge.GetNonce(), challenge.GetRealm(), nil)
        self.ua.IncLCSeq()
        self.ka_tr, err = self.ua.PrepTr(req)
        if err == nil {
            self.triedauth = true
        }
        self.ua.SipTM().BeginClientTransaction(req, self.ka_tr)
        return
    }
    if code == 407 && resp.GetSipProxyAuthenticate() != nil && self.ua.GetUsername() != "" && self.ua.GetPassword() != "" && ! self.triedauth {
        challenge := resp.GetSipProxyAuthenticate()
        req := self.ua.GenRequest("INVITE", self.ua.GetLSDP(), challenge.GetNonce(), challenge.GetRealm(), sippy_header.NewSipProxyAuthorization)
        self.ua.IncLCSeq()
        self.ka_tr, err = self.ua.PrepTr(req)
        if err == nil {
            self.triedauth = true
        }
        self.ua.SipTM().BeginClientTransaction(req, self.ka_tr)
        return
    }
    if code < 200 {
        return
    }
    self.ka_tr = nil
    self.keepalives += 1
    if code == 408 || code == 481 || code == 486 {
        if self.keepalives == 1 {
            //print "%s: Remote UAS at %s:%d does not support re-INVITES, disabling keep alives" % (self.ua.cId, self.ua.rAddr[0], self.ua.rAddr[1])
            StartTimeout(func() { self.ua.Disconnect(nil) }, self.ua.GetSessionLock(), 600, 1, self.ua.Config().ErrorLogger())
            return
        }
        //print "%s: Received %d response to keep alive from %s:%d, disconnecting the call" % (self.ua.cId, code, self.ua.rAddr[0], self.ua.rAddr[1])
        self.ua.Disconnect(nil)
        return
    }
    StartTimeout(self.keepAlive, self.ua.GetSessionLock(), self.ua.GetKaInterval(), 1, self.ua.Config().ErrorLogger())
}

func (self *keepaliveController) keepAlive() {
    var err error
    if _, ok := self.ua.GetState().(*UaStateConnected); ! ok {
        return
    }
    req := self.ua.GenRequest("INVITE", self.ua.GetLSDP(), "", "", nil)
    self.ua.IncLCSeq()
    self.triedauth = false
    self.ka_tr, err = self.ua.PrepTr(req)
    if err == nil {
        self.ua.SipTM().BeginClientTransaction(req, self.ka_tr)
    }
}
