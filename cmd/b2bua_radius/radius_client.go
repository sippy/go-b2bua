//
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
    "strings"
)

type RadiusClient struct {
    external_command    *ExternalCommand
    _avpair_names       map[string]bool
    _cisco_vsa_names    map[string]bool
}

func NewRadiusClient(global_config *myConfigParser) *RadiusClient {
    var external_command *ExternalCommand

    if global_config.Radiusclient_conf != "" {
        external_command = newExternalCommand(global_config.Max_radius_clients, global_config.ErrorLogger(), global_config.Radiusclient, "-f", global_config.Radiusclient_conf, "-s")
    } else {
        external_command = newExternalCommand(global_config.Max_radius_clients, global_config.ErrorLogger(), global_config.Radiusclient, "-s")
    }
    return &RadiusClient{
        external_command    : external_command,
        _avpair_names       : map[string]bool {
            "call-id"                   : true,
            "h323-session-protocol"     : true,
            "h323-ivr-out"              : true,
            "h323-incoming-conf-id"     : true,
            "release-source"            : true,
            "alert-timepoint"           : true,
            "provisional-timepoint"     : true,
        },
        _cisco_vsa_names    : map[string]bool {
            "h323-remote-address"       : true,
            "h323-conf-id"              : true,
            "h323-setup-time"           : true,
            "h323-call-origin"          : true,
            "h323-call-type"            : true,
            "h323-connect-time"         : true,
            "h323-disconnect-time"      : true,
            "h323-disconnect-cause"     : true,
            "h323-voice-quality"        : true,
            "h323-credit-time"          : true,
            "h323-return-code"          : true,
            "h323-redirect-number"      : true,
            "h323-preferred-lang"       : true,
            "h323-billing-model"        : true,
            "h323-currency"             : true,
        },
    }
}

func (self *RadiusClient) _prepare_attributes(typ string, attributes []RadiusAttribute) []string {
    data := []string{ typ }
    var a, v string
    for _, attr := range attributes {
        if _, ok := self._avpair_names[attr.name]; ok {
            v = fmt.Sprintf("%s=%s", attr.name, attr.value)
            a = "Cisco-AVPair"
        } else if _, ok := self._cisco_vsa_names[attr.name]; ok {
            v = fmt.Sprintf("%s=%s", attr.name, attr.value)
            a = attr.name
        } else {
            a = attr.name
            v = attr.value
        }
        data = append(data, fmt.Sprintf("%s=\"%s\"", a, v))
    }
    return data
}

func (self *RadiusClient) do_auth(attributes []RadiusAttribute, result_callback func(results *RadiusResult)) Cancellable {
    return self.external_command.process_command(self._prepare_attributes("AUTH", attributes), func(results []string) { self.process_result(results, result_callback) })
}

func (self *RadiusClient) do_acct(attributes []RadiusAttribute, result_callback func(results *RadiusResult) /*= nil*/) {
    self.external_command.process_command(self._prepare_attributes("ACCT", attributes), func (results []string) { self.process_result(results, result_callback) })
}

func (self *RadiusClient) process_result(results []string, result_callback func(*RadiusResult)) {
    if result_callback == nil {
        return
    }
    result := NewRadiusResult()
    if len(results) > 0 {
        for _, r := range results[:len(results)-1] {
            av := strings.SplitN(r, " = ", 2)
            if len(av) != 2 {
                continue
            }
            attr := av[0]
            val := strings.Trim(av[1], "'")
            if _, ok := self._cisco_vsa_names[attr]; ok || attr == "Cisco-AVPair" {
                av = strings.SplitN(val, "=", 2)
                if len(av) > 1 {
                    attr = av[0]
                    val = av[1]
                }
            } else if strings.HasPrefix(val, attr + "=") {
                val = val[len(attr) + 1:]
            }
            result.Avps = append(result.Avps, RadiusAttribute{ attr, val })
        }
    }
    result_callback(result)
}
