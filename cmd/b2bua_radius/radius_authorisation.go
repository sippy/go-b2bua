// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2024 Sippy Software, Inc. All rights reserved.
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
package main

import (
    "fmt"
    "time"

    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/net"
)

type RadiusAuthorisation struct {
    radius_client   *RadiusClient
    global_config   *myConfigParser
}

func NewRadiusAuthorisation(radius_client *RadiusClient, global_config *myConfigParser) *RadiusAuthorisation {
    return &RadiusAuthorisation{
        radius_client   : radius_client,
        global_config   : global_config,
    }
}

func (self *RadiusAuthorisation) Do_auth(username, caller, callee string, h323_cid *sippy_header.SipCiscoGUID,
      sip_cid *sippy_header.SipCallId, remote_ip *sippy_net.MyAddress, res_cb func(*RadiusResult),
      realm, nonce, uri, response string, extra_attributes ...RadiusAttribute) Cancellable {
    var attributes []RadiusAttribute
    if realm != "" && nonce != "" && uri != "" && response != "" {
        attributes = []RadiusAttribute{
            { "User-Name", username },
            { "Digest-Realm", realm },
            { "Digest-Nonce", nonce },
            { "Digest-Method", "INVITE" },
            { "Digest-URI", uri },
            { "Digest-Algorithm", "MD5" },
            { "Digest-User-Name", username },
            { "Digest-Response", response},
        }
    } else {
        attributes = []RadiusAttribute{
            { "User-Name", remote_ip.String() },
            { "Password", "cisco" },
        }
    }
    attributes = append(attributes, []RadiusAttribute{
        { "Calling-Station-Id", caller },
        { "Called-Station-Id", callee },
        { "h323-conf-id", h323_cid.StringBody() },
        { "call-id", sip_cid.StringBody() },
        { "h323-remote-address", remote_ip.String() },
        { "h323-session-protocol", "sipv2"},
    }...)
    attributes = append(attributes, extra_attributes...)
    message := "sending AAA request:\n"
    for _, attr := range attributes {
        message += fmt.Sprintf("%-32s = '%s'\n", attr.name, attr.value)
    }
    self.global_config.SipLogger().Write(nil, sip_cid.StringBody(), message)
    return self.radius_client.do_auth(attributes, func(results *RadiusResult) { self._process_result(results, res_cb, sip_cid.StringBody(), time.Now()) })
}

func (self *RadiusAuthorisation) _process_result(results *RadiusResult, res_cb func(*RadiusResult), sip_cid string, btime time.Time) {
    var message string

    delay := time.Now().Sub(btime)
    if results != nil && results.Rcode == 0 || results.Rcode == 1 {
        if results.Rcode == 0 {
            message = fmt.Sprintf("AAA request accepted (delay is %.3f), processing response:\n", delay.Seconds())
        } else {
            message = fmt.Sprintf("AAA request rejected (delay is %.3f), processing response:\n", delay.Seconds())
        }
        for _, res := range results.Avps {
            message += fmt.Sprintf("%-32s = '%s'\n", res.name, res.value)
        }
    } else {
        message = fmt.Sprintf("Error sending AAA request (delay is %.3f)\n", delay.Seconds())
    }
    self.global_config.SipLogger().Write(nil, sip_cid, message)
    res_cb(results)
}
