// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2025 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2016 Andriy Pylypenko. All rights reserved.
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

package rtp_proxy_session

import (
    "strings"
    "strconv"

    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/exceptions"
)

type UpdateParams struct {
    rtpps           *Rtp_proxy_session
    remote_ip       string
    remote_port     string
    result_callback func(*UpdateResult, *Rtp_proxy_session, sippy_types.SipHandlingError)
    options         string
    index           int
    atype           string
    subcommands     []*Subcommand
}

type UpdateResult struct {
    rtpproxy_address    string
    rtpproxy_port       int
    family              string
    sendonly            bool
}

type Subcommand struct {
    commands []string
    handleResults func([]string, *UpdateResult) (sippy_types.SipHandlingError)
}

func NewUpdateParams(rtpps *Rtp_proxy_session, index int, result_cb func(*UpdateResult, *Rtp_proxy_session, sippy_types.SipHandlingError)) *UpdateParams {
    return &UpdateParams{
        rtpps:           rtpps,
        index:           index,
        result_callback: result_cb,
        remote_ip:       "0.0.0.0",
        remote_port:     "0",
        atype:           "IP4",
        subcommands:     []*Subcommand{},
    }
}

func (up *UpdateParams) ProcessRtppResult(result string) *UpdateResult {
    if result == "" {
        ex := sippy_exceptions.NewRtpProxyError("RTPProxy errored")
        up.result_callback(nil, up.rtpps, ex)
        return nil
    }

    result = strings.TrimSpace(result)
    t0 := strings.SplitN(result, "&&", 2)
    t1 := strings.Fields(t0[0])

    if len(t1) == 0 || t1[0][0] == 'E' {
        ex := sippy_exceptions.NewRtpProxyError("RTPProxy errored: " + t1[0])
        up.result_callback(nil, up.rtpps, ex)
        return nil
    }

    ur := &UpdateResult{}

    if len(up.subcommands) > 0 {
        subcRess := []string{}
        if len(t0) > 1 {
            subcRess = strings.Split(t0[1], "&&")
            for i := range subcRess {
                subcRess[i] = strings.TrimSpace(subcRess[i])
            }
        }

        actual := len(subcRess)
        expected := 0
        for _, subc := range up.subcommands {
            expected += len(subc.commands)
        }

        if actual > expected {
            ex := sippy_exceptions.NewRtpProxyError("RTPProxy errored: too many results")
            up.result_callback(nil, up.rtpps, ex)
            return nil
        }

        if actual > 0 && subcRess[len(subcRess)-1] == "-1" {
            foff := len(subcRess)
            for _, subc := range up.subcommands {
                if foff > len(subc.commands) {
                    foff -= len(subc.commands)
                    continue
                }
                ex := sippy_exceptions.NewRtpProxyError("RTPProxy errored: " + subc.commands[foff-1] + ": -1")
                up.result_callback(nil, up.rtpps, ex)
                return nil
            }
        }

        if actual < expected {
            extra := make([]string, expected - actual)
            for i := range extra {
                extra[i] = "0"
            }
            subcRess = append(subcRess, extra...)
        }

        for _, subc := range up.subcommands {
            results := subcRess[:len(subc.commands)]
            if ex := subc.handleResults(results, ur); ex != nil {
                up.result_callback(nil, up.rtpps, ex)
                return nil
            }
            subcRess = subcRess[len(subc.commands):]
            if len(subcRess) == 0 {
                break
            }
        }
    }

    var err error
    ur.rtpproxy_port, err = strconv.Atoi(t1[0])
    if err != nil || ur.rtpproxy_port == 0 {
        ex := sippy_exceptions.NewRtpProxyError("RTPProxy errored: bad port")
        up.result_callback(nil, up.rtpps, ex)
        return nil
    }

    ur.family = "IP4"
    if len(t1) > 1 {
        ur.rtpproxy_address = t1[1]
        if len(t1) > 2 && t1[2] == "6" {
            ur.family = "IP6"
        }
    } else {
        ur.rtpproxy_address = up.rtpps._rtp_proxy_client.GetProxyAddress()
    }

    if up.atype == "IP4" && up.remote_ip == "0.0.0.0" {
        ur.sendonly = true
    } else if up.atype == "IP6" && up.remote_ip == "::" {
        ur.sendonly = true
    } else {
        ur.sendonly = false
    }

    up.result_callback(ur, up.rtpps, nil)
    return ur
}

func (self *UpdateResult) Address() string {
    return self.rtpproxy_address
}
