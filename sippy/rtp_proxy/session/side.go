// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2014 Sippy Software, Inc. All rights reserved.
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
    "strconv"
    "strings"
    "sync/atomic"

    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/sdp"
    "github.com/sippy/go-b2bua/sippy/types"
)

type _rtpps_side struct {
    otherside       *_rtpps_side
    session_exists  bool
    laddress        string
    raddress        *sippy_net.HostPort
    codecs          string
    repacketize     int
    after_sdp_change func(sippy_types.RtpProxyUpdateResult)
    from_tag        string
    to_tag          string
}

func (self *_rtpps_side) _play(prompt_name string, times int, result_callback func(string), index int, rtpps *Rtp_proxy_session) {
    if ! self.session_exists {
        return
    }
    if ! self.otherside.session_exists {
        up_cb := func(*UpdateResult, *Rtp_proxy_session, sippy_types.SipHandlingError) { self.__play(prompt_name, times, result_callback, index, rtpps) }
        up := NewUpdateParams(rtpps, index, up_cb)
        self.otherside.update(up)
        return
    }
    self.__play(prompt_name, times, result_callback, index, rtpps)
}

func (self *_rtpps_side) __play(prompt_name string, times int, result_callback func(string), index int, rtpps *Rtp_proxy_session) {
    command := "P" + strconv.Itoa(times) + " " + rtpps.call_id + "-" + strconv.Itoa(index) + " " + prompt_name + " " + self.codecs + " " + self.from_tag + " " + self.to_tag
    rtpps.send_command(command, func(r string) { rtpps.command_result(r, result_callback) })
}

func max(a, b int) int {
     if a >= b {return a}
     return b
}

func (self *_rtpps_side) update(up *UpdateParams) {
    var sbind_supported, is_local, tnot_supported bool
    var err error

    command := "U"
    up.rtpps.max_index = max(up.rtpps.max_index, up.index)
    if sbind_supported, err = up.rtpps.SBindSupported(); err != nil {
        return
    }
    if is_local, err = up.rtpps.IsLocal(); err != nil {
        return
    }
    if tnot_supported, err = up.rtpps.TNotSupported(); err != nil {
        return
    }
    if sbind_supported {
        if self.raddress != nil {
            //if self.owner.IsLocal() && up.atype == "IP4" {
            //    options += "L" + self.laddress
            //} else if ! self.owner.IsLocal() {
            //    options += "R" + self.raddress.Host.String()
            //}
            up.options += "R" + self.raddress.Host.String()
        } else if self.laddress != "" && is_local {
            up.options += "L" + self.laddress
        }
    }
    command += up.options
    if self.otherside.session_exists {
        command += " " + up.rtpps.call_id + "-" + strconv.Itoa(up.index) + " " + up.remote_ip + " " + up.remote_port + " " + self.from_tag + " " + self.to_tag
    } else {
        command += " " + up.rtpps.call_id + "-" + strconv.Itoa(up.index) + " " + up.remote_ip + " " + up.remote_port + " " + self.from_tag
    }
    if up.rtpps.notify_socket != "" && up.index == 0 && tnot_supported {
        command += " " + up.rtpps.notify_socket + " " + up.rtpps.notify_tag
    }
    up.rtpps.send_command(command, func(r string) { self.update_result(r, up) })
}

func (self *_rtpps_side) update_result(result string, up *UpdateParams) {
    //print "%s.update_result(%s)" % (id(self), result)
    //result_callback, face, callback_parameters = args
    self.session_exists = true
    up.ProcessRtppResult(result)
}

func (self *_rtpps_side) _on_sdp_change(rtpps *Rtp_proxy_session, sdp_body sippy_types.MsgBody, result_callback sippy_types.OnDelayedCB) error {
    parsed_body, err := sdp_body.GetSdp()
    if err != nil {
        return err
    }
    sects := []*sippy_sdp.SdpMediaDescription{}
    for _, sect := range parsed_body.GetSections() {
        switch strings.ToLower(sect.GetMHeader().GetTransport()) {
        case "udp", "udptl", "rtp/avp", "rtp/savp", "udp/bfcp":
            sects = append(sects, sect)
        default:
        }
    }
    if len(sects) == 0 {
        sdp_body.SetNeedsUpdate(false)
        result_callback(sdp_body, nil)
        return nil
    }
    formats := sects[0].GetMHeader().GetFormats()
    self.codecs = strings.Join(formats, ",")
    options := ""
    if self.repacketize > 0 {
        options = "z" + strconv.Itoa(self.repacketize)
    }
    sections_left := int64(len(sects))
    for i, sect := range sects {
        sect_options := options
        if sect.GetCHeader().GetAType() == "IP6" {
            sect_options = "6" + options
        }
        up_cb := func (ur *UpdateResult, rtpps *Rtp_proxy_session, ex sippy_types.SipHandlingError) { self._sdp_change_finish(sdp_body, parsed_body, sect, &sections_left, result_callback, ur, rtpps, ex) }
        up := NewUpdateParams(rtpps, i, up_cb)
        up.remote_ip = sect.GetCHeader().GetAddr()
        up.remote_port = sect.GetMHeader().GetPort()
        up.atype = sect.GetCHeader().GetAType()
        up.options = sect_options

        self.update(up)
    }
    return nil
}

func (self *_rtpps_side) _sdp_change_finish(sdp_body sippy_types.MsgBody, parsed_body sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription, sections_left *int64, result_callback sippy_types.OnDelayedCB, ur *UpdateResult, rtpps *Rtp_proxy_session, ex sippy_types.SipHandlingError) {
    if ! sdp_body.NeedsUpdate() {
        return
    }
    if ex != nil {
        sdp_body.SetNeedsUpdate(false)
        result_callback(nil, ex)
        return
    }
    if ur != nil {
        if self.after_sdp_change != nil {
            self.after_sdp_change(ur)
        }
        sect.GetCHeader().SetAType(ur.family)
        sect.GetCHeader().SetAddr(ur.rtpproxy_address)
        if sect.GetMHeader().GetPort() != "0" {
            sect.GetMHeader().SetPort(strconv.Itoa(ur.rtpproxy_port))
        }
        if ur.sendonly {
            sect.RemoveAHeader("sendrecv")
            if ! sect.HasAHeader([]string{ "recvonly", "sendonly", "inactive" }) {
                sect.AddHeader("a", "sendonly")
            }
        }
        if self.repacketize > 0 {
            sect.RemoveAHeader("ptime:")
            sect.AddHeader("a", "ptime:" + strconv.Itoa(self.repacketize))
        }
    }
    if atomic.AddInt64(sections_left, -1) > 0 {
        // more work is in progress
        return
    }
    if rtpps.insert_nortpp {
        parsed_body.AppendAHeader("nortpproxy=yes")
    }
    sdp_body.SetNeedsUpdate(false)
    // RFC4566
    // *******
    // For privacy reasons, it is sometimes desirable to obfuscate the
    // username and IP address of the session originator.  If this is a
    // concern, an arbitrary <username> and private <unicast-address> MAY be
    // chosen to populate the "o=" field, provided that these are selected
    // in a manner that does not affect the global uniqueness of the field.
    // *******
    origin := parsed_body.GetOHeader()
    origin.SetAddress("192.0.2.1")
    origin.SetAddressType("IP4")
    origin.SetNetworkType("IN")
    result_callback(sdp_body, nil)
}

func (self *_rtpps_side) _stop_play(cb func(string), index int, rtpps *Rtp_proxy_session) {
    if ! self.otherside.session_exists {
        return
    }
    command := "S " + rtpps.call_id + "-" + strconv.Itoa(index) + " " + self.from_tag + " " + self.to_tag
    rtpps.send_command(command, func(r string) { rtpps.command_result(r, cb) })
}
