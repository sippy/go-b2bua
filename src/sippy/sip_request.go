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
    "strings"

    "sippy/conf"
    "sippy/headers"
    "sippy/time"
    "sippy/types"
)

type sipRequest struct {
    sipMsg
    method  string
    sipver  string
    ruri    *sippy_header.SipURL
    expires *sippy_header.SipExpires
    user_agent *sippy_header.SipUserAgent
}

func ParseSipRequest(buf []byte, rtime *sippy_time.MonoTime) (*sipRequest, error) {
    self := &sipRequest{}
    super, err := ParseSipMsg(buf, self, rtime)
    self.sipMsg = *super
    if err != nil {
        return nil, err
    }
    arr := strings.Fields(self.startline)
    if len(arr) != 3 {
        return nil, errors.New("SIP bad start line in SIP request: " + self.startline)
    }
    self.method, self.sipver = arr[0], arr[2]
    self.ruri, err = sippy_header.ParseSipURL(arr[1], false /* relaxedparser */)
    if err != nil {
        return nil, errors.New("Bad SIP URL in SIP request: " + arr[1])
    }
    err = self.init_body()
    if err != nil {
        if e, ok := err.(*ESipParseException); ok {
            e.sip_response = self.GenResponse(400, "Bad Request - " + e.Error(), nil, nil)
        }
    }
    return self, err
}

func NewSipRequest(method string, ruri *sippy_header.SipURL, sipver string, to *sippy_header.SipTo,
        from *sippy_header.SipFrom, via *sippy_header.SipVia, cseq int, callid *sippy_header.SipCallId,
        maxforwards *sippy_header.SipMaxForwards, body sippy_types.MsgBody, contact *sippy_header.SipContact,
        routes []*sippy_header.SipRoute, target *sippy_conf.HostPort, cguid *sippy_header.SipCiscoGUID,
        user_agent *sippy_header.SipUserAgent, expires *sippy_header.SipExpires, config sippy_conf.Config) *sipRequest {
    if routes == nil {
        routes = make([]*sippy_header.SipRoute, 0)
    }
    self := &sipRequest{}
    self.sipMsg = *NewSipMsg(self, nil)
    self.method = method
    self.ruri = ruri
    if target == nil {
        if len(routes) == 0 {
            self.SetTarget(self.ruri.GetAddr(config))
        } else {
            self.SetTarget(routes[0].GetAddr(config))
        }
    } else {
        self.SetTarget(target)
    }
    if sipver == "" {
        self.sipver = "SIP/2.0"
    } else {
        self.sipver = sipver
    }
    if via == nil {
        self.AppendHeader(sippy_header.NewSipVia(config))
        self.vias[0].GenBranch()
    } else {
        self.AppendHeader(via)
    }
    for _, route := range routes {
        self.AppendHeader(route)
    }
    if maxforwards == nil   { maxforwards = sippy_header.NewSipMaxForwards() }
    self.AppendHeader(maxforwards)
    if from == nil          { from = sippy_header.NewSipFrom(nil, config) }
    self.AppendHeader(from)
    if to == nil            { to = sippy_header.NewSipTo(sippy_header.NewSipAddress("", ruri), config) }
    self.AppendHeader(to)
    if callid == nil        { callid = sippy_header.NewSipCallId(config) }
    self.AppendHeader(callid)
    self.AppendHeader(sippy_header.NewSipCSeq(cseq, method))
    if contact != nil {
        self.AppendHeader(contact)
    }
    if expires == nil && method == "INVITE" {
        self.AppendHeader(sippy_header.NewSipExpires())
    } else if expires != nil {
        self.AppendHeader(expires)
    }
    if user_agent != nil {
        self.user_agent = user_agent
    } else {
        self.user_agent = sippy_header.NewSipUserAgent(config.GetMyUAName())
    }
    self.AppendHeader(self.user_agent)
    if cguid != nil {
        self.AppendHeader(cguid)
        self.AppendHeader(cguid.AsH323ConfId())
    }
    self.setBody(body)
    return self
}

func (self *sipRequest) GetTo() *sippy_header.SipTo {
    return self.to
}

func (self *sipRequest) getSL() string {
    return self.method + " " + self.ruri.String() + " " + self.sipver
}

func (self *sipRequest) GetMethod() string {
    return self.method
}

func (self *sipRequest) GetRURI() *sippy_header.SipURL {
    return self.ruri
}

func (self *sipRequest) SetRURI(ruri *sippy_header.SipURL) {
    self.ruri = ruri
}

func (self *sipRequest) GenResponse(scode int, reason string, body sippy_types.MsgBody, server *sippy_header.SipServer) sippy_types.SipResponse {
    // Should be done at the transaction level
    // to = self.getHF('to').getBody().getCopy()
    // if code > 100 and to.getTag() == None:
    //    to.genTag()
    vias := make([]*sippy_header.SipVia, 0)
    rrs := make([]*sippy_header.SipRecordRoute, 0)
    for _, via := range self.vias {
        vias = append(vias, via.GetCopy())
    }
    for _, rr := range self.record_routes {
        rrs = append(rrs, rr.GetCopy())
    }
    return NewSipResponse(scode, reason, self.sipver, self.from.GetCopy(),
                       self.call_id.GetCopy(), vias, self.to.GetCopy(),
                       self.cseq.GetCopy(), rrs, body, server)
}

func (self *sipRequest) GenACK(to *sippy_header.SipTo, config sippy_conf.Config) sippy_types.SipRequest {
    if to == nil {
        to = self.to.GetCopy()
    }
    var maxforwards *sippy_header.SipMaxForwards = nil

    if len(self.maxforwards) > 0 {
        maxforwards = self.maxforwards[0].GetCopy()
    } else {
        maxforwards = nil
    }
    return NewSipRequest("ACK", /*ruri =*/ self.ruri.GetCopy(), /*sipver =*/ self.sipver,
                      /*to =*/ to, /*from =*/ self.from.GetCopy(), /*via =*/ self.vias[0].GetCopy(),
                      /*cseq =*/ self.cseq.CSeq, /*callid =*/ self.call_id.GetCopy(),
                      /*maxforwards =*/ maxforwards, /*body*/ nil, /*contact*/ nil,
                      /*routes*/ nil, /*target*/ nil, /*cguid*/ nil, /*user_agent =*/ self.user_agent,
                      /*expires*/ nil, config)
}

func (self *sipRequest) GenCANCEL(config sippy_conf.Config) sippy_types.SipRequest {
    var maxforwards *sippy_header.SipMaxForwards = nil

    if len(self.maxforwards) > 0 {
        maxforwards = self.maxforwards[0].GetCopy()
    }
    routes := make([]*sippy_header.SipRoute, len(self.routes))
    for i, r := range self.routes {
        routes[i] = r.GetCopy()
    }
    return NewSipRequest("CANCEL", /*ruri*/ self.ruri.GetCopy(), /*sipver*/ self.sipver,
                      /*to*/ self.to.GetCopy(), /*from*/ self.from.GetCopy(), /*via*/ self.vias[0].GetCopy(),
                      /*cseq*/ self.cseq.CSeq, /*callid*/ self.call_id.GetCopy(),
                      /*maxforwards*/ maxforwards, /*body*/ nil, /*contact*/ nil,
                      /*routes*/ routes, /*target*/ self.GetTarget(), /*cguid*/ nil,
                      /*user_agent*/ self.user_agent, /*expires*/ nil, config)
}

func (self *sipRequest) GetExpires() *sippy_header.SipExpires {
    return self.expires
}
