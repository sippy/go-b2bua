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
    "fmt"

    "sippy/conf"
    "sippy/headers"
    "sippy/types"
)

type UacStateUpdating struct {
    *uaStateGeneric
    triedauth   bool
}

func NewUacStateUpdating(ua sippy_types.UA, config sippy_conf.Config) *UacStateUpdating {
    self := &UacStateUpdating{
        uaStateGeneric  : newUaStateGeneric(ua, config),
        triedauth       : false,
    }
    self.connected = true
    return self
}

func (self *UacStateUpdating) String() string {
    return "Updating(UAC)"
}

func (self *UacStateUpdating) RecvRequest(req sippy_types.SipRequest, t sippy_types.ServerTransaction) (sippy_types.UaState, func()) {
    if req.GetMethod() == "INVITE" {
        t.SendResponse(req.GenResponse(491, "Request Pending", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        return nil, nil
    } else if req.GetMethod() == "BYE" {
        self.ua.GetClientTransaction().Cancel()
        t.SendResponse(req.GenResponse(200, "OK", nil, self.ua.GetLocalUA().AsSipServer()), false, nil)
        //print "BYE received in the Updating state, going to the Disconnected state"
        event := NewCCEventDisconnect(nil, req.GetRtime(), self.ua.GetOrigin())
        event.SetReason(req.GetReason())
        self.ua.Enqueue(event)
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(req.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(req.GetRtime(), self.ua.GetOrigin(), 0, nil) }
    }
    //print "wrong request %s in the state Updating" % req.getMethod()
    return nil, nil
}

func (self *UacStateUpdating) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) (sippy_types.UaState, func()) {
    var err error
    var event sippy_types.CCEvent

    body := resp.GetBody()
    code, reason := resp.GetSCode()
    if code < 200 {
        self.ua.Enqueue(NewCCEventRing(code, reason, body, resp.GetRtime(), self.ua.GetOrigin()))
        return nil, nil
    }
    if code >= 200 && code < 300 {
        if ! self.ua.GetLateMedia() || body == nil {
            event = NewCCEventConnect(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
        } else {
            event = NewCCEventPreConnect(code, reason, body, resp.GetRtime(), self.ua.GetOrigin())
            tr.SetUAck(true)
            self.ua.SetPendingTr(tr)
        }
        if body != nil {
            if self.ua.HasOnRemoteSdpChange() {
                if err := self.ua.OnRemoteSdpChange(body, func (x sippy_types.MsgBody) { self.ua.DelayedRemoteSdpUpdate(event, x) }); err != nil {
                    ev := NewCCEventFail(502, "Bad Gateway", event.GetRtime(), "")
                    ev.SetWarning(fmt.Sprintf("Malformed SDP Body received from downstream: \"%s\"", err.Error()))
                    return self.updateFailed(ev)
                }
                return NewUaStateConnected(self.ua, self.config), nil
            } else {
                self.ua.SetRSDP(body.GetCopy())
            }
        } else {
            self.ua.SetRSDP(nil)
        }
        self.ua.Enqueue(event)
        return NewUaStateConnected(self.ua, self.config), nil
    }
    reason_rfc3326 := resp.GetReason()
    if (code == 301 || code == 302) && len(resp.GetContacts()) > 0 {
        var contact *sippy_header.SipAddress

        contact, err = resp.GetContacts()[0].GetBody(self.config)
        if err != nil {
            self.config.ErrorLogger().Error("UacStateUpdating::RecvResponse: #1: " + err.Error())
            return nil, nil
        }
        event = NewCCEventRedirect(code, reason, body,
                    []*sippy_header.SipAddress{ contact.GetCopy() },
                    resp.GetRtime(), self.ua.GetOrigin())
    } else if code == 300 && len(resp.GetContacts()) > 0 {
        urls := make([]*sippy_header.SipAddress, 0)
        for _, contact := range resp.GetContacts() {
            var cbody *sippy_header.SipAddress

            cbody, err = contact.GetBody(self.config)
            if err != nil {
                self.config.ErrorLogger().Error("UacStateUpdating::RecvResponse: #2: " + err.Error())
                return nil, nil
            }
            urls = append(urls, cbody.GetCopy())
        }
        event = NewCCEventRedirect(code, reason, body, urls, resp.GetRtime(), self.ua.GetOrigin())
    } else {
        event = NewCCEventFail(code, reason, resp.GetRtime(), self.ua.GetOrigin())
        event.SetReason(reason_rfc3326)
    }
    if code == 408 || code == 481 {
        // (Call/Transaction Does Not Exist) or a 408 (Request Timeout), the
        // UAC SHOULD terminate the dialog.  A UAC SHOULD also terminate a
        // dialog if no response at all is received for the request (the
        // client transaction would inform the TU about the timeout.)
        return self.updateFailed(event)
    }
    self.ua.Enqueue(event)
    return NewUaStateConnected(self.ua, self.config), nil
}

func (self *UacStateUpdating) updateFailed(event sippy_types.CCEvent) (sippy_types.UaState, func()) {
    self.ua.Enqueue(event)
    eh := []sippy_header.SipHeader{}
    if event.GetReason() != nil {
        eh = append(eh, event.GetReason())
    }
    req, err := self.ua.GenRequest("BYE", nil, nil, eh...)
    if err != nil {
        self.config.ErrorLogger().Error("UacStateUpdating::updateFailed: #1: " + err.Error())
        return nil, nil
    }
    self.ua.SipTM().BeginNewClientTransaction(req, nil, self.ua.GetSessionLock(), self.ua.GetSourceAddress(), nil, self.ua.BeforeRequestSent)

    self.ua.CancelCreditTimer()
    self.ua.SetDisconnectTs(event.GetRtime())
    event = NewCCEventDisconnect(nil, event.GetRtime(), self.ua.GetOrigin())
    self.ua.Enqueue(event)
    return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(event.GetRtime(), event.GetOrigin(), 0, nil) }
}

func (self *UacStateUpdating) RecvEvent(event sippy_types.CCEvent) (sippy_types.UaState, func(), error) {
    send_bye := false
    switch event.(type) {
    case *CCEventDisconnect:    send_bye = true
    case *CCEventFail:          send_bye = true
    case *CCEventRedirect:      send_bye = true
    }
    if send_bye {
        self.ua.GetClientTransaction().Cancel()
        req, err := self.ua.GenRequest("BYE", nil, nil, event.GetExtraHeaders()...)
        if err != nil {
            return nil, nil, err
        }
        self.ua.SipTM().BeginNewClientTransaction(req, nil, self.ua.GetSessionLock(), self.ua.GetSourceAddress(), nil, self.ua.BeforeRequestSent)
        self.ua.CancelCreditTimer()
        self.ua.SetDisconnectTs(event.GetRtime())
        return NewUaStateDisconnected(self.ua, self.config), func() { self.ua.DiscCb(event.GetRtime(), event.GetOrigin(), 0, nil) }, nil
    }
    //return nil, fmt.Errorf("wrong event %s in the Updating state", event.String())
    return nil, nil, nil
}

func (self *UacStateUpdating) ID() sippy_types.UaStateID {
    return sippy_types.UAC_STATE_UPDATING
}
