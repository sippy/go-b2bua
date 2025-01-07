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
    "strconv"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/utils"
)

type sipResponse struct {
    *sipMsg
    scode int
    reason string
    sipver string
}

func ParseSipResponse(buf []byte, rtime *sippy_time.MonoTime, config sippy_conf.Config) (*sipResponse, error) {
    var scode string
    var err error
    var super *sipMsg

    self := &sipResponse{}
    super, err = ParseSipMsg(buf, rtime, config)
    if err != nil {
        return nil, err
    }
    self.sipMsg = super
    // parse startline
    sstartline := sippy_utils.FieldsN(self.startline, 3)
    if len(sstartline) == 2 {
        // Some brain-damaged UAs don't include reason in some cases
        self.sipver, scode = sstartline[0], sstartline[1]
        self.reason = "Unspecified"
    } else if len(sstartline) == 3 {
        self.sipver, scode, self.reason = sstartline[0], sstartline[1], sstartline[2]
    } else {
        return nil, errors.New("Bad response: " + self.startline)
    }
    self.scode, err = strconv.Atoi(scode)
    if err != nil {
        return nil, err
    }
    if self.scode != 100 || self.scode < 400 {
        err = self.init_body(config.ErrorLogger())
    }
    return self, err
}

func NewSipResponse(scode int, reason, sipver string, from *sippy_header.SipFrom, callid *sippy_header.SipCallId,
        vias []*sippy_header.SipVia, to *sippy_header.SipTo, cseq *sippy_header.SipCSeq, rrs []*sippy_header.SipRecordRoute,
        body sippy_types.MsgBody, server *sippy_header.SipServer, config sippy_conf.Config) *sipResponse {
    self := &sipResponse{
        scode : scode,
        reason : reason,
        sipver : sipver,
    }
    self.sipMsg = NewSipMsg(nil, config)
    for _, via := range vias {
        self.AppendHeader(via)
    }
    for _, rr := range rrs {
        self.AppendHeader(rr)
    }
    self.AppendHeader(from)
    self.AppendHeader(to)
    self.AppendHeader(callid)
    self.AppendHeader(cseq)
    if server != nil {
        self.AppendHeader(server)
    }
    self.body = body
    return self
}

func (self *sipResponse) LocalStr(hostport *sippy_net.HostPort, compact bool /*= False*/ ) string {
    return self.GetSL() + "\r\n" + self.localStr(hostport, compact)
}

func (self *sipResponse) GetSL() string {
    return self.sipver + " " + strconv.Itoa(self.scode) + " " + self.reason
}

func (self *sipResponse) GetCopy() sippy_types.SipResponse {
    rval := &sipResponse{
        scode   : self.scode,
        reason  : self.reason,
        sipver  : self.sipver,
    }
    rval.sipMsg = self.sipMsg.getCopy()
    return rval
}

func (self *sipResponse) GetSCode() (int, string) {
    return self.scode, self.reason
}

func (self *sipResponse) SetSCode(scode int, reason string) {
    self.scode, self.reason = scode, reason
}

func (self *sipResponse) GetSCodeNum() int {
    return self.scode
}

func (self *sipResponse) GetSCodeReason() string {
    return self.reason
}

func (self *sipResponse) SetSCodeReason(reason string) {
    self.reason = reason
}

func (self *sipResponse) GetRTId() (*sippy_header.RTID, error) {
    if self.rseq == nil {
        return nil, errors.New("No RSeq present")
    }
    rseq, err := self.rseq.GetBody()
    if err != nil {
        return nil, errors.New("Error parsing RSeq: " + err.Error())
    }
    cseq, err := self.cseq.GetBody()
    if err != nil {
        return nil, errors.New("Error parsing CSeq: " + err.Error())
    }
    call_id := self.call_id.StringBody()
    from_body, err := self.from.GetBody(self.config)
    if err != nil {
        return nil, errors.New("Error parsing From: " + err.Error())
    }
    from_tag := from_body.GetTag()
    return sippy_header.NewRTID(call_id, from_tag, rseq.Number, cseq.CSeq, cseq.Method), nil
}

func (self *sipResponse) GetChallenges() []sippy_types.Challenge {
    res := make([]sippy_types.Challenge, 0, len(self.sip_www_authenticates) + len(self.sip_proxy_authenticates))
    for _, challenge := range self.sip_www_authenticates {
        res = append(res, challenge)
    }
    for _, challenge := range self.sip_proxy_authenticates {
        res = append(res, challenge)
    }
    return res
}
