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
    "strconv"
    "time"

    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

var sipErrToH323Err = map[int][2]string{
    400 : { "7f", "Interworking, unspecified" },
    401 : { "39", "Bearer capability not authorized" },
    402 : { "15", "Call rejected" },
    403 : { "39", "Bearer capability not authorized" },
    404 : { "1", "Unallocated number" },
    405 : { "7f", "Interworking, unspecified" },
    406 : { "7f", "Interworking, unspecified" },
    407 : { "15", "Call rejected" },
    408 : { "66", "Recover on Expires timeout" },
    409 : { "29", "Temporary failure" },
    410 : { "1", "Unallocated number" },
    411 : { "7f", "Interworking, unspecified" },
    413 : { "7f", "Interworking, unspecified" },
    414 : { "7f", "Interworking, unspecified" },
    415 : { "4f", "Service or option not implemented" },
    420 : { "7f", "Interworking, unspecified" },
    480 : { "12", "No user response" },
    481 : { "7f", "Interworking, unspecified" },
    482 : { "7f", "Interworking, unspecified" },
    483 : { "7f", "Interworking, unspecified" },
    484 : { "1c", "Address incomplete" },
    485 : { "1", "Unallocated number" },
    486 : { "11", "User busy" },
    487 : { "12", "No user responding" },
    488 : { "7f", "Interworking, unspecified" },
    500 : { "29", "Temporary failure" },
    501 : { "4f", "Service or option not implemented" },
    502 : { "26", "Network out of order" },
    503 : { "3f", "Service or option unavailable" },
    504 : { "66", "Recover on Expires timeout" },
    505 : { "7f", "Interworking, unspecified" },
    580 : { "2f", "Resource unavailable, unspecified" },
    600 : { "11", "User busy" },
    603 : { "15", "Call rejected" },
    604 : { "1",  "Unallocated number" },
    606 : { "3a", "Bearer capability not presently available"},
}

type RadiusAccounting struct {
    crec            bool
    drec            bool
    iTime           *sippy_time.MonoTime
    cTime           *sippy_time.MonoTime
    lperiod         int
    p1xx_ts         *sippy_time.MonoTime
    p100_ts         *sippy_time.MonoTime
    send_start      bool
    user_agent      string
    _attributes     []RadiusAttribute
    el              chan bool
    complete        bool
    ms_precision    bool
    origin          string
    global_config   *myConfigParser
    sip_cid         string
    radius_client   *RadiusClient
}

func NewRadiusAccounting(global_config *myConfigParser, origin string, radius_client *RadiusClient) *RadiusAccounting {
    return &RadiusAccounting{
        crec            : false,
        drec            : false,
        lperiod         : global_config.Alive_acct_int,
        send_start      : global_config.Start_acct_enable,
        _attributes     : []RadiusAttribute{
            { "h323-call-origin", origin },
            { "h323-call-type", "VoIP" },
            { "h323-session-protocol", "sipv2" },
        },
        el              : make(chan bool, 1),
        complete        : false,
        origin          : origin,
        global_config   : global_config,
        radius_client   : radius_client,
    }
}

func (self *RadiusAccounting) SetParams(username, caller, callee, h323_cid, sip_cid, remote_ip, h323_in_cid string) {
    self._attributes = append(self._attributes, []RadiusAttribute{
        { "User-Name", username },
        { "Calling-Station-Id", caller },
        { "Called-Station-Id", callee },
        { "h323-conf-id", h323_cid },
        { "call-id", sip_cid },
        { "Acct-Session-Id", sip_cid },
        { "h323-remote-address", remote_ip },
    }...)
    if h323_in_cid != "" && h323_in_cid != h323_cid {
        self._attributes = append(self._attributes, RadiusAttribute{ "h323-incoming-conf-id", h323_in_cid })
    }
    self.sip_cid = sip_cid
    self.complete = true
}

func (self *RadiusAccounting) Conn(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string) {
    if self.crec {
        return
    }
    self.crec = true
    self.iTime = ua.GetSetupTs()
    self.cTime = ua.GetConnectTs()
    if self.user_agent == "" {
        self.user_agent = ua.GetRemoteUA()
    }
    if ua.GetP1xxTs() != nil {
        self.p1xx_ts = ua.GetP1xxTs()
    }
    if ua.GetP100Ts() != nil {
        self.p100_ts = ua.GetP100Ts()
    }
    if self.send_start {
        self.asend("Start", rtime, origin, 0, ua)
    }
    self._attributes = append(self._attributes, []RadiusAttribute{
        { "h323-voice-quality", "0" },
        { "Acct-Terminate-Cause", "User-Request" },
    }...)
    if self.lperiod > 0 {
        self.el = make(chan bool)
        go func() {
            for {
                select {
                case <-self.el:
                    return
                case <-time.After(time.Duration(self.lperiod) * time.Second):
                    self.asend("Alive", nil, "", 0, nil)
                }
            }
        }()
    }
}

func (self *RadiusAccounting) Disc(ua sippy_types.UA, rtime *sippy_time.MonoTime, origin string, result int/*= 0*/) {
    if self.drec {
        return
    }
    self.drec = true
    if self.el != nil {
        close(self.el)
    }
    if self.iTime == nil {
        self.iTime = ua.GetSetupTs()
    }
    if self.cTime == nil {
        self.cTime = rtime
    }
    if ua.GetRemoteUA() != "" && self.user_agent == "" {
        self.user_agent = ua.GetRemoteUA()
    }
    if ua.GetP1xxTs() != nil {
        self.p1xx_ts = ua.GetP1xxTs()
    }
    if ua.GetP100Ts() != nil {
        self.p100_ts = ua.GetP100Ts()
    }
    self.asend("Stop", rtime, origin, result, ua)
}

func (self *RadiusAccounting) asend(typ string, rtime *sippy_time.MonoTime /*= nil*/, origin string /*= nil*/, result int /*= 0*/, ua sippy_types.UA /*= nil*/) {
    var duration, delay time.Duration
    //var connected bool

    if ! self.complete {
        return
    }
    if rtime == nil {
        rtime, _ = sippy_time.NewMonoTime()
    }
    if ua != nil {
        duration, delay, _ /*connected*/, _ = ua.GetAcct(rtime)
    } else {
        // Alive accounting
        duration = rtime.Sub(self.cTime)
        delay = self.cTime.Sub(self.iTime)
        //connected = true
    }
    if ! self.global_config.Precise_acct {
        duration = duration.Round(time.Second)
        delay = delay.Round(time.Second)
    }
    attributes := make([]RadiusAttribute, len(self._attributes))
    copy(attributes, self._attributes)
    if typ != "Start" {
        var dc string

        if result >= 400 {
            res, ok := sipErrToH323Err[result]
            if ok {
                dc = res[0]
            } else {
                dc = "7f"
            }
        } else if result < 200 {
            dc = "10"
        } else {
            dc = "0"
        }
        attributes = append(attributes, []RadiusAttribute{
            { "h323-disconnect-time", self.ftime(self.iTime.Realt().Add(delay).Add(duration)) },
            { "Acct-Session-Time", strconv.Itoa(int(duration.Round(time.Second).Seconds())) },
            { "h323-disconnect-cause", dc },
        }...)
    }
    if typ == "Stop" {
        release_source := "8"
        if origin == "caller" {
            release_source = "2"
        } else if origin == "callee" {
            release_source = "4"
        }
        attributes = append(attributes, RadiusAttribute{ "release-source", release_source })
    }
    attributes = append(attributes, []RadiusAttribute{
        { "h323-connect-time", self.ftime(self.iTime.Realt().Add(delay)) },
        { "h323-setup-time", self.ftime(self.iTime.Realt()) },
        { "Acct-Status-Type", typ },
    }...)
    if self.user_agent != "" {
        attributes = append(attributes, RadiusAttribute{ "h323-ivr-out", "sip_ua:" + self.user_agent })
    }
    if self.p1xx_ts != nil {
        attributes = append(attributes, RadiusAttribute{ "Acct-Delay-Time", strconv.Itoa(int(self.p1xx_ts.Realt().Round(time.Second).Unix())) })
    }
    if self.p100_ts != nil {
        attributes = append(attributes, RadiusAttribute{ "provisional-timepoint", self.ftime(self.p100_ts.Realt()) })
    }
    pattributes := fmt.Sprintf("sending Acct %s (%s):\n", typ, self.origin/*.capitalize()*/)
    for _, attr := range attributes {
        pattributes += fmt.Sprintf("%-32s = '%s'\n", attr.name, attr.value)
    }
    self.global_config.SipLogger().Write(rtime, self.sip_cid, pattributes)
    self.radius_client.do_acct(attributes, func(results *RadiusResult) { self._process_result(results, self.sip_cid, time.Now()) })
}

func (self *RadiusAccounting) ftime(t time.Time) string {
    if self.ms_precision {
        return t.In(time.FixedZone("GMT", 0)).Format("15:04:05.000 MST Mon Jan 2 2006")
    }
    return t.In(time.FixedZone("GMT", 0)).Format("15:04:05 MST Mon Jan 2 2006")
}

func (self *RadiusAccounting) _process_result(results *RadiusResult, sip_cid string, btime time.Time) {
    var message string

    delay := time.Now().Sub(btime)
    if results != nil && (results.Rcode == 0 || results.Rcode == 1) {
        if results.Rcode == 0 {
            message = fmt.Sprintf("Acct/%s request accepted (delay is %.3f)\n", self.origin, delay.Seconds())
        } else {
            message = fmt.Sprintf("Acct/%s request rejected (delay is %.3f)\n", self.origin, delay.Seconds())
        }
    } else {
        message = fmt.Sprintf("Error sending Acct/%s request (delay is %.3f)\n", self.origin, delay.Seconds())
    }
    self.global_config.SipLogger().Write(nil, sip_cid, message)
}
