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
    "math"
    "strconv"
    "strings"
    "sync/atomic"

    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/sdp"
    "github.com/sippy/go-b2bua/sippy/types"
)

type _rtpps_side struct {
    otherside       *_rtpps_side
    owner           *Rtp_proxy_session
    session_exists  bool
    laddress        string
    raddress        *sippy_net.HostPort
    codecs          string
    repacketize     int
    after_sdp_change func(sippy_types.RtpProxyUpdateResult)
    from_tag        string
    to_tag          string
}

func (self *_rtpps_side) _play(prompt_name string, times int, result_callback func(string), index int) {
    if ! self.session_exists {
        return
    }
    if ! self.otherside.session_exists {
        self.otherside.update("0.0.0.0", "0", func(*rtpproxy_update_result) { self.__play(prompt_name, times, result_callback, index) }, "", index, "IP4")
        return
    }
    self.__play(prompt_name, times, result_callback, index)
}

func (self *_rtpps_side) __play(prompt_name string, times int, result_callback func(string), index int) {
    command := "P" + strconv.Itoa(times) + " " + self.owner.call_id + "-" + strconv.Itoa(index) + " " + prompt_name + " " + self.codecs + " " + self.from_tag + " " + self.to_tag
    self.owner.send_command(command, func(r string) { self.owner.command_result(r, result_callback) })
}

func (self *_rtpps_side) update(remote_ip string, remote_port string, result_callback func(*rtpproxy_update_result), options/*= ""*/ string, index /*= 0*/int, atype /*= "IP4"*/string) {
    var sbind_supported, is_local, tnot_supported bool
    var err error

    command := "U"
    self.owner.max_index = int(math.Max(float64(self.owner.max_index), float64(index)))
    if sbind_supported, err = self.owner.SBindSupported(); err != nil {
        return
    }
    if is_local, err = self.owner.IsLocal(); err != nil {
        return
    }
    if tnot_supported, err = self.owner.TNotSupported(); err != nil {
        return
    }
    if sbind_supported {
        if self.raddress != nil {
            //if self.owner.IsLocal() && atype == "IP4" {
            //    options += "L" + self.laddress
            //} else if ! self.owner.IsLocal() {
            //    options += "R" + self.raddress.Host.String()
            //}
            options += "R" + self.raddress.Host.String()
        } else if self.laddress != "" && is_local {
            options += "L" + self.laddress
        }
    }
    command += options
    if self.otherside.session_exists {
        command += " " + self.owner.call_id + "-" + strconv.Itoa(index) + " " + remote_ip + " " + remote_port + " " + self.from_tag + " " + self.to_tag
    } else {
        command += " " + self.owner.call_id + "-" + strconv.Itoa(index) + " " + remote_ip + " " + remote_port + " " + self.from_tag
    }
    if self.owner.notify_socket != "" && index == 0 && tnot_supported {
        command += " " + self.owner.notify_socket + " " + self.owner.notify_tag
    }
    self.owner.send_command(command, func(r string) { self.update_result(r, remote_ip, atype, result_callback) })
}

func (self *_rtpps_side) update_result(result, remote_ip, atype string, result_callback func(*rtpproxy_update_result)) {
    //print "%s.update_result(%s)" % (id(self), result)
    //result_callback, face, callback_parameters = args
    self.session_exists = true
    if result == "" {
        result_callback(nil)
        return
    }
    t1 := strings.Fields(result)
    if t1[0][0] == 'E' {
        result_callback(nil)
        return
    }
    rtpproxy_port, err := strconv.Atoi(t1[0])
    if err != nil || rtpproxy_port == 0 {
        result_callback(nil)
        return
    }
    family := "IP4"
    rtpproxy_address := ""
    if len(t1) > 1 {
        rtpproxy_address = t1[1]
        if len(t1) > 2 && t1[2] == "6" {
            family = "IP6"
        }
    } else {
        if rtpproxy_address, err = self.owner.GetProxyAddress(); err != nil {
            return
        }
    }
    sendonly := false
    if atype == "IP4" && remote_ip == "0.0.0.0" {
        sendonly = true
    } else if atype == "IP6" && remote_ip == "::" {
        sendonly = true
    }
    result_callback(&rtpproxy_update_result{
        rtpproxy_address    : rtpproxy_address,
        rtpproxy_port       : t1[0],
        family              : family,
        sendonly            : sendonly,
    })
}

func (self *_rtpps_side) _on_sdp_change(sdp_body sippy_types.MsgBody, result_callback func(sippy_types.MsgBody)) error {
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
        result_callback(sdp_body)
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
        self.update(sect.GetCHeader().GetAddr(), sect.GetMHeader().GetPort(),
              func (res *rtpproxy_update_result) { self._sdp_change_finish(res, sdp_body, parsed_body, sect, &sections_left, result_callback) },
              sect_options, i, sect.GetCHeader().GetAType())
    }
    return nil
}

func (self *_rtpps_side) _sdp_change_finish(cb_args *rtpproxy_update_result, sdp_body sippy_types.MsgBody, parsed_body sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription, sections_left *int64, result_callback func(sippy_types.MsgBody)) {
    if cb_args != nil {
        if self.after_sdp_change != nil {
            self.after_sdp_change(cb_args)
        }
        sect.GetCHeader().SetAType(cb_args.family)
        sect.GetCHeader().SetAddr(cb_args.rtpproxy_address)
        if sect.GetMHeader().GetPort() != "0" {
            sect.GetMHeader().SetPort(cb_args.rtpproxy_port)
        }
        if cb_args.sendonly {
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
    if self.owner.insert_nortpp {
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
    result_callback(sdp_body)
}

func (self *_rtpps_side) _stop_play(cb func(string), index int) {
    if ! self.otherside.session_exists {
        return
    }
    command := "S " + self.owner.call_id + "-" + strconv.Itoa(index) + " " + self.from_tag + " " + self.to_tag
    self.owner.send_command(command, func(r string) { self.owner.command_result(r, cb) })
}
