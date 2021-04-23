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
    "errors"
    "fmt"
    "strconv"
    "strings"

    "sippy/conf"
    "sippy/headers"
    "sippy/log"
    "sippy/net"
    "sippy/time"
    "sippy/types"
)

type sipMsg struct {
    headers             []sippy_header.SipHeader
    __mbody             *string
    startline           string
    vias                []*sippy_header.SipVia
    contacts            []*sippy_header.SipContact
    to                  *sippy_header.SipTo
    from                *sippy_header.SipFrom
    cseq                *sippy_header.SipCSeq
    rseq                *sippy_header.SipRSeq
    rack                *sippy_header.SipRAck
    content_length      *sippy_header.SipContentLength
    content_type        *sippy_header.SipContentType
    call_id             *sippy_header.SipCallId
    refer_to            *sippy_header.SipReferTo
    maxforwards         *sippy_header.SipMaxForwards
    also                []*sippy_header.SipAlso
    rtime               *sippy_time.MonoTime
    body                sippy_types.MsgBody
    source              *sippy_net.HostPort
    record_routes       []*sippy_header.SipRecordRoute
    routes              []*sippy_header.SipRoute
    target              *sippy_net.HostPort
    reason_hf           *sippy_header.SipReason
    sip_warning         *sippy_header.SipWarning
    sip_www_authenticates []*sippy_header.SipWWWAuthenticate
    sip_authorization   *sippy_header.SipAuthorization
    sip_proxy_authorization *sippy_header.SipProxyAuthorization
    sip_proxy_authenticates []*sippy_header.SipProxyAuthenticate
    sip_server          *sippy_header.SipServer
    sip_user_agent      *sippy_header.SipUserAgent
    sip_cisco_guid      *sippy_header.SipCiscoGUID
    sip_h323_conf_id    *sippy_header.SipH323ConfId
    sip_require         []*sippy_header.SipRequire
    sip_supported       []*sippy_header.SipSupported
    sip_date            *sippy_header.SipDate
    config              sippy_conf.Config
}

func NewSipMsg(rtime *sippy_time.MonoTime, config sippy_conf.Config) *sipMsg {
    self := &sipMsg{
        headers         : make([]sippy_header.SipHeader, 0),
        __mbody         : nil,
        vias            : make([]*sippy_header.SipVia, 0),
        contacts        : make([]*sippy_header.SipContact, 0),
        record_routes   : make([]*sippy_header.SipRecordRoute, 0),
        routes          : make([]*sippy_header.SipRoute, 0),
        also            : make([]*sippy_header.SipAlso, 0),
        sip_require     : make([]*sippy_header.SipRequire, 0),
        sip_supported   : make([]*sippy_header.SipSupported, 0),
        rtime           : rtime,
        config          : config,
    }
    return self
}

func ParseSipMsg(_buf []byte, rtime *sippy_time.MonoTime, config sippy_conf.Config) (*sipMsg, error) {
    self := NewSipMsg(rtime, config)
    buf := string(_buf)
    // Locate a body
    for _, bdel := range []string{ "\r\n\r\n", "\r\r", "\n\n" } {
        boff := strings.Index(buf, bdel)
        if boff != -1 {
            tmp := buf[boff + len(bdel):]
            self.__mbody = &tmp
            buf = buf[:boff]
            if len(*self.__mbody) == 0 {
                self.__mbody = nil
            }
            break
        }
    }
    // Split message into lines and put aside start line
    lines := strings.FieldsFunc(buf, func(c rune) bool { return c == '\n' || c == '\r' })
    self.startline = lines[0]
    header_lines := make([]string, 0)
    prev_l := ""
    for _, l := range lines[1:] {
        if l == "" || l[0] == ' ' || l[0] == '\t' {
            prev_l += strings.TrimSpace(l)
        } else {
            if len(prev_l) > 0 {
                header_lines = append(header_lines, prev_l)
            }
            prev_l = l
        }
    }
    if prev_l != "" {
        header_lines = append(header_lines, prev_l)
    }

    // Parse headers
    for _, line := range header_lines {
        headers, err := ParseSipHeader(line)
        if err != nil {
            return nil, err
        }
        for _, header := range headers {
            if contact, ok := header.(*sippy_header.SipContact); ok {
                if contact.Asterisk {
                    continue
                }
            }
            self.AppendHeader(header)
        }
    }
    if len(self.vias) == 0 {
        return nil, errors.New("Via HF is missed")
    }
    if self.to == nil {
        return nil, errors.New("To HF is missed")
    }
    if self.from == nil {
        return nil, errors.New("From HF is missed")
    }
    if self.cseq == nil {
        return nil, errors.New("CSeq HF is missed")
    }
    if self.call_id == nil {
        return nil, errors.New("Call-ID HF is missed")
    }
    return self, nil
}

func (self *sipMsg) appendHeaders(hdrs []sippy_header.SipHeader) {
    if hdrs == nil { return }
    for _, hdr := range hdrs {
        self.AppendHeader(hdr)
    }
}

func (self *sipMsg) AppendHeader(hdr sippy_header.SipHeader) {
    switch t := hdr.(type) {
    case *sippy_header.SipCSeq:
        self.cseq = t
    case *sippy_header.SipRSeq:
        self.rseq = t
    case *sippy_header.SipRAck:
        self.rack = t
    case *sippy_header.SipCallId:
        self.call_id = t
    case *sippy_header.SipFrom:
        self.from = t
    case *sippy_header.SipTo:
        self.to = t
    case *sippy_header.SipMaxForwards:
        self.maxforwards = t
        return
    case *sippy_header.SipVia:
        self.vias = append(self.vias, t)
        return
    case *sippy_header.SipContentLength:
        self.content_length = t
        return
    case *sippy_header.SipContentType:
        self.content_type = t
        return
    case *sippy_header.SipExpires:
    case *sippy_header.SipRecordRoute:
        self.record_routes = append(self.record_routes, t)
    case *sippy_header.SipRoute:
        self.routes = append(self.routes, t)
        return
    case *sippy_header.SipContact:
        self.contacts = append(self.contacts, t)
    case *sippy_header.SipWWWAuthenticate:
        self.sip_www_authenticates = append(self.sip_www_authenticates, t)
    case *sippy_header.SipAuthorization:
        self.sip_authorization = t
        self.sip_proxy_authorization = nil
        return
    case *sippy_header.SipServer:
        self.sip_server = t
    case *sippy_header.SipUserAgent:
        self.sip_user_agent = t
    case *sippy_header.SipCiscoGUID:
        self.sip_cisco_guid = t
    case *sippy_header.SipH323ConfId:
        self.sip_h323_conf_id = t
    case *sippy_header.SipAlso:
        self.also = append(self.also, t)
    case *sippy_header.SipReferTo:
        self.refer_to = t
    case *sippy_header.SipCCDiversion:
    case *sippy_header.SipReferredBy:
    case *sippy_header.SipProxyAuthenticate:
        self.sip_proxy_authenticates = append(self.sip_proxy_authenticates, t)
    case *sippy_header.SipProxyAuthorization:
        self.sip_proxy_authorization = t
        self.sip_authorization = nil
        return
    case *sippy_header.SipReplaces:
    case *sippy_header.SipReason:
        self.reason_hf  = t
    case *sippy_header.SipWarning:
        self.sip_warning = t
    case *sippy_header.SipRequire:
        self.sip_require = append(self.sip_require, t)
    case *sippy_header.SipSupported:
        self.sip_supported = append(self.sip_supported, t)
    case *sippy_header.SipDate:
        self.sip_date = t
    case nil:
        return
    }
    self.headers = append(self.headers, hdr)
}

func (self *sipMsg) init_body(logger sippy_log.ErrorLogger) error {
    var blen_hf *sippy_header.SipNumericHF
    if self.content_length != nil {
        blen_hf, _ = self.content_length.GetBody()
    }
    if blen_hf != nil {
        blen := blen_hf.Number
        mblen := 0
        if self.__mbody != nil {
            mblen = len([]byte(*self.__mbody)) // length in bytes, not runes
        }
        if blen == 0 {
            self.__mbody = nil
            mblen = 0
        } else if self.__mbody == nil {
            // XXX: Should generate 400 Bad Request if such condition
            // happens with request
            return &ESipParseException{ msg : fmt.Sprintf("Missed SIP body, %d bytes expected", blen) }
        } else if blen > mblen {
            if blen - mblen < 7 && mblen > 7 && (*self.__mbody)[len(*self.__mbody)-4:] == "\r\n\r\n" {
                // XXX: we should not really be doing this, but it appears to be
                // a common off-by-one/two/.../six problem with SDPs generates by
                // the consumer-grade devices.
                logger.Debugf("Truncated SIP body, %d bytes expected, %d received, fixing...", blen, mblen)
                blen = mblen
            } else if blen - mblen == 2 && (*self.__mbody)[len(*self.__mbody)-2:] == "\r\n" {
                // Missed last 2 \r\n is another common problem.
                logger.Debugf("Truncated SIP body, %d bytes expected, %d received, fixing...", blen, mblen)
                (*self.__mbody) += "\r\n"
            } else if blen - mblen == 1 && (*self.__mbody)[len(*self.__mbody)-3:] == "\r\n\n" {
                // Another possible mishap
                logger.Debugf("Truncated SIP body, %d bytes expected, %d received, fixing...", blen, mblen)
                (*self.__mbody) = (*self.__mbody)[:len(*self.__mbody)-3] + "\r\n\r\n"
            } else if blen - mblen == 1 && (*self.__mbody)[len(*self.__mbody)-2:] == "\r\n" {
                // One more
                logger.Debugf("Truncated SIP body, %d bytes expected, %d received, fixing...", blen, mblen)
                (*self.__mbody) += "\r\n"
                blen += 1
                mblen += 2
            } else {
                // XXX: Should generate 400 Bad Request if such condition
                // happens with request
                return &ESipParseException{ msg : fmt.Sprintf("Truncated SIP body, %d bytes expected, %d received", blen, mblen) }
            }
        } else if blen < mblen {
            *self.__mbody = (*self.__mbody)[:blen]
            mblen = blen
        }
    }
    if self.__mbody != nil {
        if self.content_type != nil {
            self.body = NewMsgBody(*self.__mbody, strings.ToLower(self.content_type.StringBody()))
        } else {
            self.body = NewMsgBody(*self.__mbody, "application/sdp")
        }
    }
    return nil
}

func (self *sipMsg) Bytes() []byte {
    s := self.startline + "\r\n"
    for _, via := range self.vias {
        s += via.String() + "\r\n"
    }
    for _, via := range self.routes {
        s += via.String() + "\r\n"
    }
    if self.maxforwards != nil {
        s += self.maxforwards.String() + "\r\n"
    }
    for _, header := range self.headers {
        s += header.String() + "\r\n"
    }
    mbody := []byte{}
    if self.body != nil {
        mbody = []byte(self.body.String())
        s += "Content-Length: " + strconv.Itoa(len(mbody)) + "\r\n"
        s += "Content-Type: " + self.body.GetMtype() + "\r\n\r\n"
    } else {
        s += "Content-Length: 0\r\n\r\n"
    }
    ret := []byte(s)
    ret = append(ret, mbody...)
    return ret
}

func (self *sipMsg) localStr(hostport *sippy_net.HostPort, compact bool /*= False*/ ) string {
    s := ""
    for _, via := range self.vias {
        s += via.LocalStr(hostport, compact) + "\r\n"
    }
    for _, via := range self.routes {
        s += via.LocalStr(hostport, compact) + "\r\n"
    }
    if self.maxforwards != nil {
        s += self.maxforwards.LocalStr(hostport, compact) + "\r\n"
    }
    for _, header := range self.headers {
        s += header.LocalStr(hostport, compact) + "\r\n"
    }
    if self.sip_authorization != nil {
        s += self.sip_authorization.LocalStr(hostport, compact) + "\r\n"
    } else if self.sip_proxy_authorization != nil {
        s += self.sip_proxy_authorization.LocalStr(hostport, compact) + "\r\n"
    }
    if self.body != nil {
        mbody := self.body.LocalStr(hostport)
        bmbody := []byte(mbody)
        if compact {
            s += fmt.Sprintf("l: %d\r\n", len(bmbody))
            s += "c: " + self.body.GetMtype() + "\r\n\r\n"
        } else {
            s += fmt.Sprintf("Content-Length: %d\r\n", len(bmbody))
            s += "Content-Type: " + self.body.GetMtype() + "\r\n\r\n"
        }
        s += mbody
    } else {
        if compact {
            s += "l: 0\r\n\r\n"
        } else {
            s += "Content-Length: 0\r\n\r\n"
        }
    }
    return s
}

func (self *sipMsg) setBody(body sippy_types.MsgBody) {
    self.body = body
}

func (self *sipMsg) GetBody() sippy_types.MsgBody {
    return self.body
}

func (self *sipMsg) SetBody(body sippy_types.MsgBody) {
    self.body = body
}

func (self *sipMsg) GetTarget() *sippy_net.HostPort {
    return self.target
}

func (self *sipMsg) SetTarget(address *sippy_net.HostPort) {
    self.target = address
}

func (self *sipMsg) GetSource() *sippy_net.HostPort {
    return self.source
}

func (self *sipMsg) GetTId(wCSM, wBRN, wTTG bool) (*sippy_header.TID, error) {
    var call_id, cseq, cseq_method, from_tag, to_tag, via_branch string
    var cseq_hf *sippy_header.SipCSeqBody
    var from_hf *sippy_header.SipAddress
    var err error

    if self.call_id != nil {
        call_id = self.call_id.CallId
    }
    if self.cseq == nil {
        return nil, errors.New("no CSeq: field")
    }
    if cseq_hf, err = self.cseq.GetBody(); err != nil {
        return nil, err
    }
    cseq = strconv.Itoa(cseq_hf.CSeq)
    if self.from != nil {
        if from_hf, err = self.from.GetBody(self.config); err != nil {
            return nil, err
        }
        from_tag = from_hf.GetTag()
    }
    if wCSM {
        cseq_method = cseq_hf.Method
    }
    if wBRN {
        if len(self.vias) > 0 {
            var via0 *sippy_header.SipViaBody
            via0, err = self.vias[0].GetBody()
            if err != nil {
                return nil, err
            }
            via_branch = via0.GetBranch()
        }
    }
    if wTTG {
        var to_hf *sippy_header.SipAddress
        to_hf, err = self.to.GetBody(self.config)
        if err != nil {
            return nil, err
        }
        to_tag = to_hf.GetTag()
    }
    return sippy_header.NewTID(call_id, cseq, cseq_method, from_tag, to_tag, via_branch), nil
}

func (self *sipMsg) getTIds() ([]*sippy_header.TID, error) {
    var call_id, cseq, method, ftag string
    var from_hf *sippy_header.SipAddress
    var cseq_hf *sippy_header.SipCSeqBody
    var err error

    call_id = self.call_id.CallId
    from_hf, err = self.from.GetBody(self.config)
    if err != nil {
        return nil, err
    }
    ftag = from_hf.GetTag()
    if self.cseq == nil {
        return nil, errors.New("no CSeq: field")
    }
    if cseq_hf, err = self.cseq.GetBody(); err != nil {
        return nil, err
    }
    if cseq_hf != nil {
        cseq, method = strconv.Itoa(cseq_hf.CSeq), cseq_hf.Method
    }
    ret := []*sippy_header.TID{}
    for _, via := range self.vias {
        var via_hf *sippy_header.SipViaBody

        if via_hf, err = via.GetBody(); err != nil {
            return nil, err
        }
        ret = append(ret, sippy_header.NewTID(call_id, cseq, method, ftag, via_hf.GetBranch(), ""))
    }
    return ret, nil
}

func (self *sipMsg) getCopy() *sipMsg {
    cself := NewSipMsg(self.rtime, self.config)
    for _, header := range self.vias {
        cself.AppendHeader(header.GetCopyAsIface())
    }
    for _, header := range self.routes {
        cself.AppendHeader(header.GetCopyAsIface())
    }
    for _, header := range self.headers {
        cself.AppendHeader(header.GetCopyAsIface())
    }
    if self.body != nil {
        cself.body = self.body.GetCopy()
    }
    cself.startline = self.startline
    cself.target = self.target
    cself.source = self.source
    return cself
}

func (self *sipMsg) GetSipProxyAuthorization() *sippy_header.SipProxyAuthorization {
    return self.sip_proxy_authorization
}

func (self *sipMsg) GetSipServer() *sippy_header.SipServer {
    return self.sip_server
}

func (self *sipMsg) GetSipUserAgent() *sippy_header.SipUserAgent {
    return self.sip_user_agent
}

func (self *sipMsg) GetCSeq() *sippy_header.SipCSeq {
    return self.cseq
}

func (self *sipMsg) GetRSeq() *sippy_header.SipRSeq {
    return self.rseq
}

func (self *sipMsg) GetSipRAck() *sippy_header.SipRAck {
    return self.rack
}

func (self *sipMsg) GetSipProxyAuthenticates() []*sippy_header.SipProxyAuthenticate {
    return self.sip_proxy_authenticates
}

func (self *sipMsg) GetSipWWWAuthenticates() []*sippy_header.SipWWWAuthenticate {
    return self.sip_www_authenticates
}

func (self *sipMsg) GetTo() *sippy_header.SipTo {
    return self.to
}

func (self *sipMsg) GetReason() *sippy_header.SipReason {
    return self.reason_hf
}

func (self *sipMsg) GetVias() []*sippy_header.SipVia {
    return self.vias
}

func (self *sipMsg) GetCallId() *sippy_header.SipCallId {
    return self.call_id
}

func (self *sipMsg) SetRtime(rtime *sippy_time.MonoTime) {
    self.rtime = rtime
}

func (self *sipMsg) InsertFirstVia(via *sippy_header.SipVia) {
    self.vias = append([]*sippy_header.SipVia{ via }, self.vias...)
}

func (self *sipMsg) RemoveFirstVia() {
    self.vias = self.vias[1:]
}

func (self *sipMsg) SetRoutes(routes []*sippy_header.SipRoute) {
    self.routes = routes
}

func (self *sipMsg) GetFrom() *sippy_header.SipFrom {
    return self.from
}

func (self *sipMsg) GetReferTo() *sippy_header.SipReferTo {
    return self.refer_to
}

func (self *sipMsg) GetRtime() *sippy_time.MonoTime {
    return self.rtime
}

func (self *sipMsg) GetAlso() []*sippy_header.SipAlso {
    return self.also
}

func (self *sipMsg) GetContacts() []*sippy_header.SipContact {
    return self.contacts
}

func (self *sipMsg) GetRecordRoutes() []*sippy_header.SipRecordRoute {
    return self.record_routes
}

func (self *sipMsg) GetCGUID() *sippy_header.SipCiscoGUID {
    return self.sip_cisco_guid
}

func (self *sipMsg) GetH323ConfId() *sippy_header.SipH323ConfId {
    return self.sip_h323_conf_id
}

func (self *sipMsg) GetSipAuthorization() *sippy_header.SipAuthorization {
    return self.sip_authorization
}

func match_name (name string, hf sippy_header.SipHeader) bool {
    return strings.ToLower(hf.Name()) == strings.ToLower(name) ||
        strings.ToLower(hf.CompactName()) == strings.ToLower(name)
}

func (self *sipMsg) GetFirstHF(name string) sippy_header.SipHeader {
    for _, hf := range self.headers {
        if match_name(name, hf) {
            return hf
        }
    }
    if self.content_length != nil && match_name(name, self.content_length) {
        return self.content_length
    }
    if self.content_type != nil && match_name(name, self.content_type) {
        return self.content_type
    }
    if len(self.vias) > 0 && match_name(name, self.vias[0]) {
        return self.vias[0]
    }
    if len(self.routes) > 0 && match_name(name, self.routes[0]) {
        return self.routes[0]
    }
    return nil
}

func (self *sipMsg) GetHFs(name string) []sippy_header.SipHeader {
    rval := make([]sippy_header.SipHeader, 0)
    for _, hf := range self.headers {
        if match_name(name, hf) {
            rval = append(rval, hf)
        }
    }
    if self.content_length != nil && match_name(name, self.content_length) {
        rval = append(rval, self.content_length)
    }
    if self.content_type != nil && match_name(name, self.content_type) {
        rval = append(rval, self.content_type)
    }
    if len(self.vias) > 0 && match_name(name, self.vias[0]) {
        for _, via := range self.vias {
            rval = append(rval, via)
        }
    }
    if len(self.routes) > 0 && match_name(name, self.routes[0]) {
        for _, route := range self.routes {
            rval = append(rval, route)
        }
    }
    return rval
}

func (self *sipMsg) GetMaxForwards() *sippy_header.SipMaxForwards {
    return self.maxforwards
}

func (self *sipMsg) SetMaxForwards(maxforwards *sippy_header.SipMaxForwards) {
    self.maxforwards = maxforwards
}

func (self *sipMsg) GetSipRequire() []*sippy_header.SipRequire {
    return self.sip_require
}

func (self *sipMsg) GetSipSupported() []*sippy_header.SipSupported {
    return self.sip_supported
}

func (self *sipMsg) GetSipDate() *sippy_header.SipDate {
    return self.sip_date
}
