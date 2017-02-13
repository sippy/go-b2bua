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
    "sync"

    "sippy/types"
    "sippy/time"
    "sippy/headers"
)

var global_event_seq int64 = 1
var global_event_seq_lock sync.Mutex

type CCEventGeneric struct {
    seq             int64
    rtime           *sippy_time.MonoTime
    extra_headers   []sippy_header.SipHeader
    origin          string
    sip_reason      *sippy_header.SipReason
    sip_max_forwards *sippy_header.SipMaxForwards
}

func newCCEventGeneric(rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) CCEventGeneric {
    global_event_seq_lock.Lock()
    new_seq := global_event_seq
    global_event_seq++
    global_event_seq_lock.Unlock()
    if rtime == nil {
        rtime, _ = sippy_time.NewMonoTime()
    }
    self := CCEventGeneric{
        rtime   : rtime,
        seq     : new_seq,
        origin  : origin,
        extra_headers : make([]sippy_header.SipHeader, 0, len(extra_headers)),
    }
    for _, eh := range extra_headers {
        switch header := eh.(type) {
        case *sippy_header.SipMaxForwards:
            self.sip_max_forwards = header
        case *sippy_header.SipReason:
            self.sip_reason = header
        default:
            self.extra_headers = append(self.extra_headers, eh)
        }
    }
    return self
}

func (self *CCEventGeneric) GetMaxForwards() *sippy_header.SipMaxForwards {
    return self.sip_max_forwards
}

func (self *CCEventGeneric) SetMaxForwards(max_forwards *sippy_header.SipMaxForwards) {
    self.sip_max_forwards = max_forwards
}

func (self *CCEventGeneric) AppendExtraHeader(eh sippy_header.SipHeader) {
    self.extra_headers = append(self.extra_headers, eh)
}

func (self *CCEventGeneric) GetReason() *sippy_header.SipReason {
    return self.sip_reason
}

func (self *CCEventGeneric) SetReason(sip_reason *sippy_header.SipReason) {
    self.sip_reason = sip_reason
}

func (self *CCEventGeneric) GetSeq() int64 {
    return self.seq
}

func (self *CCEventGeneric) GetRtime() *sippy_time.MonoTime {
    return self.rtime
}

func (self *CCEventGeneric) GetOrigin() string {
    return self.origin
}

func (self *CCEventGeneric) GetExtraHeaders() []sippy_header.SipHeader {
    ret := self.extra_headers
    if self.sip_reason != nil { ret = append(ret, self.sip_reason) }
    // The max_forwards should not be present here
    //if self.sip_max_forwards != nil { ret = append(ret, self.sip_max_forwards) }
    return ret
}

type CCEventTry struct {
    CCEventGeneric
    call_id     *sippy_header.SipCallId
    cisco_guid  *sippy_header.SipCiscoGUID
    cli, cld, caller_name string
    auth        *sippy_header.SipAuthorization
    body        sippy_types.MsgBody
}

func NewCCEventTry(call_id *sippy_header.SipCallId, cisco_guid *sippy_header.SipCiscoGUID, cli string, cld string, body sippy_types.MsgBody, auth *sippy_header.SipAuthorization, caller_name string, rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) *CCEventTry {
    return &CCEventTry{
        CCEventGeneric : newCCEventGeneric(rtime, origin, extra_headers...),
        call_id     : call_id,
        cli         : cli,
        cld         : cld,
        auth        : auth,
        caller_name : caller_name,
        body        : body,
    }
}

func (self *CCEventTry) GetBody() sippy_types.MsgBody {
    return self.body
}

func (self *CCEventTry) GetSipAuthorization() *sippy_header.SipAuthorization {
    return self.auth
}

func (self *CCEventTry) GetSipCallId() *sippy_header.SipCallId {
    return self.call_id
}

func (self *CCEventTry) GetSipCiscoGUID() *sippy_header.SipCiscoGUID {
    return self.cisco_guid
}

func (self *CCEventTry) GetCallerName() string {
    return self.caller_name
}

func (self *CCEventTry) GetCLD() string {
    return self.cld
}

func (self *CCEventTry) GetCLI() string {
    return self.cli
}

func (self *CCEventTry) String() string { return "CCEventTry" }

type CCEventRing struct {
    CCEventGeneric
    scode           int
    scode_reason    string
    body            sippy_types.MsgBody
}

func NewCCEventRing(scode int, scode_reason string, body sippy_types.MsgBody, rtime *sippy_time.MonoTime, origin string) *CCEventRing {
    return &CCEventRing{
        CCEventGeneric  : newCCEventGeneric(rtime, origin),
        scode           : scode,
        scode_reason    : scode_reason,
        body            : body,
    }
}

func (self *CCEventRing) String() string { return "CCEventRing" }

type CCEventConnect struct {
    CCEventGeneric
    scode           int
    scode_reason    string
    body    sippy_types.MsgBody
}

func (self *CCEventRing) GetScode() int { return self.scode }
func (self *CCEventRing) GetBody() sippy_types.MsgBody { return self.body }

func NewCCEventConnect(scode int, scode_reason string, msg_body sippy_types.MsgBody, rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) *CCEventConnect {
    return &CCEventConnect{
        CCEventGeneric : newCCEventGeneric(rtime, origin, extra_headers...),
        scode           : scode,
        scode_reason    : scode_reason,
        body            : msg_body,
    }
}

func (self *CCEventConnect) String() string { return "CCEventConnect" }

func (self *CCEventConnect) GetBody() sippy_types.MsgBody {
    return self.body
}

type CCEventUpdate struct {
    CCEventGeneric
    body    sippy_types.MsgBody
}

func NewCCEventUpdate(rtime *sippy_time.MonoTime, origin string, reason *sippy_header.SipReason, max_forwards *sippy_header.SipMaxForwards, msg_body sippy_types.MsgBody) *CCEventUpdate {
    self := &CCEventUpdate{
        CCEventGeneric  : newCCEventGeneric(rtime, origin),
        body            : msg_body,
    }
    self.SetReason(reason)
    self.SetMaxForwards(max_forwards)
    return self
}

func (self *CCEventUpdate) String() string { return "CCEventUpdate" }

func (self *CCEventUpdate) GetBody() sippy_types.MsgBody {
    return self.body
}

type CCEventInfo struct {
    CCEventGeneric
    body    sippy_types.MsgBody
}

func NewCCEventInfo(rtime *sippy_time.MonoTime, origin string, msg_body sippy_types.MsgBody, extra_headers ...sippy_header.SipHeader) *CCEventInfo {
    return &CCEventInfo{
        CCEventGeneric : newCCEventGeneric(rtime, origin, extra_headers...),
    }
}

func (self *CCEventInfo) String() string { return "CCEventInfo" }

func (self *CCEventInfo) GetBody() sippy_types.MsgBody {
    return self.body
}

type CCEventDisconnect struct {
    CCEventGeneric
    redirect_url *sippy_header.SipURL
}

func NewCCEventDisconnect(also *sippy_header.SipURL, rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) *CCEventDisconnect {
    return &CCEventDisconnect{
        CCEventGeneric  : newCCEventGeneric(rtime, origin, extra_headers...),
        redirect_url    : also,
    }
}

func (self *CCEventDisconnect) String() string { return "CCEventDisconnect" }

func (self *CCEventDisconnect) GetRedirectURL() *sippy_header.SipURL {
    return self.redirect_url
}

type CCEventFail struct {
    CCEventGeneric
    challenge       sippy_header.SipHeader
    scode           int
    scode_reason    string
    warning         *sippy_header.SipWarning
}

func NewCCEventFail(scode int, scode_reason string, rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) *CCEventFail {
    return &CCEventFail{
        CCEventGeneric  : newCCEventGeneric(rtime, origin, extra_headers...),
        scode_reason    : scode_reason,
        scode           : scode,
    }
}

func (self *CCEventFail) String() string { return "CCEventFail" }

func (self *CCEventFail) GetScode() int { return self.scode }
func (self *CCEventFail) SetScode(scode int) { self.scode = scode }
func (self *CCEventFail) GetScodeReason() string { return self.scode_reason }
func (self *CCEventFail) SetScodeReason(reason string) { self.scode_reason = reason }

func (self *CCEventFail) GetExtraHeaders() []sippy_header.SipHeader {
    if self.challenge == nil {
        return self.CCEventGeneric.GetExtraHeaders()
    }
    extra_headers := self.CCEventGeneric.GetExtraHeaders()
    if extra_headers == nil {
        return []sippy_header.SipHeader{ self.challenge }
    }
    extra_headers = append(extra_headers, self.challenge)
    return extra_headers
}

func (self *CCEventFail) SetWarning(text string) {
    self.warning = sippy_header.NewSipWarning(text)
}

type CCEventRedirect struct {
    CCEventGeneric
    redirect_url    *sippy_header.SipURL
    scode           int
    scode_reason    string
    body            sippy_types.MsgBody
}

func NewCCEventRedirect(scode int, scode_reason string, body sippy_types.MsgBody, url *sippy_header.SipURL, rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) *CCEventRedirect {
    return &CCEventRedirect{
        CCEventGeneric  : newCCEventGeneric(rtime, origin, extra_headers...),
        scode           : scode,
        scode_reason    : scode_reason,
        body            : body,
        redirect_url    : url,
    }
}

func (self *CCEventRedirect) String() string { return "CCEventRedirect" }

func (self *CCEventRedirect) GetRedirectURL() *sippy_header.SipURL {
    return self.redirect_url
}

type CCEventPreConnect struct {
    CCEventGeneric
    scode           int
    scode_reason    string
    body            sippy_types.MsgBody
}

func NewCCEventPreConnect(scode int, scode_reason string, body sippy_types.MsgBody, rtime *sippy_time.MonoTime, origin string) *CCEventPreConnect {
    return &CCEventPreConnect{
        CCEventGeneric  : newCCEventGeneric(rtime, origin),
        scode           : scode,
        scode_reason    : scode_reason,
        body            : body,
    }
}

func (self *CCEventPreConnect) String() string { return "CCEventPreConnect" }
func (self *CCEventPreConnect) GetScode() int { return self.scode }
func (self *CCEventPreConnect) GetScodeReason() string { return self.scode_reason }
