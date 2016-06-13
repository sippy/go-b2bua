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
package sippy

import (
    "fmt"
    "math"
    "strconv"
    "strings"

    "sippy/sdp"
    "sippy/conf"
    "sippy/types"
)

type _rtpps_side struct {
    otherside       *_rtpps_side
    owner           *Rtp_proxy_session
    session_exists  bool
    laddress        string
    raddress        *sippy_conf.HostPort
    codecs          string
    origin          *sippy_sdp.SdpOrigin
}

func (self *_rtpps_side) _play(prompt_name string, times int, result_callback func(string), index int) {
    if ! self.session_exists {
        return
    }
    if ! self.otherside.session_exists {
        self.otherside.update("0.0.0.0", "0", func(*rtp_command_result) { self.__play(prompt_name, times, result_callback, index) }, "", index, "IP4")
        return
    }
    self.__play(prompt_name, times, result_callback, index)
}

func (self *_rtpps_side) __play(prompt_name string, times int, result_callback func(string), index int) {
    command := fmt.Sprintf("P%d %s-%d %s %s %s %s", times, self.owner.call_id, index, prompt_name, self.codecs, self.owner.from_tag, self.owner.to_tag)
    self.owner.rtp_proxy_client.SendCommand(command, func(r string) { self.owner.command_result(r, result_callback) })
}

func (self *_rtpps_side) update(remote_ip string, remote_port string, result_callback func(*rtp_command_result), options/*= ""*/ string, index /*= 0*/int, atype /*= "IP4"*/string) {
    command := "U"
    self.owner.max_index = int(math.Max(float64(self.owner.max_index), float64(index)))
    if self.owner.rtp_proxy_client.SBindSupported() && self.raddress != nil {
        if self.owner.rtp_proxy_client.IsLocal() && atype == "IP4" {
            options += fmt.Sprintf("L%s", self.laddress)
        } else if ! self.owner.rtp_proxy_client.IsLocal() {
            options += fmt.Sprintf("R%s", self.raddress.Host.String())
        }
    }
    command += options
    if self.otherside.session_exists {
        command += fmt.Sprintf(" %s-%d %s %s %s %s", self.owner.call_id, index, remote_ip, remote_port, self.owner.from_tag, self.owner.to_tag)
    } else {
        command += fmt.Sprintf(" %s-%d %s %s %s", self.owner.call_id, index, remote_ip, remote_port, self.owner.from_tag)
    }
    if self.owner.notify_socket != "" && index == 0 && self.owner.rtp_proxy_client.TNotSupported() {
        command += fmt.Sprintf(" %s %s", self.owner.notify_socket, self.owner.notify_tag)
    }
    self.owner.rtp_proxy_client.SendCommand(command, func(r string) { self.update_result(r, result_callback) })
}

func (self *_rtpps_side) update_result(result string, result_callback func(*rtp_command_result)) {
    //print "%s.update_result(%s)" % (id(self), result)
    //result_callback, face, callback_parameters = args
    self.session_exists = true
    if result == "" {
        result_callback(nil)
        return
    }
    t1 := strings.Fields(result)
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
        rtpproxy_address = self.owner.rtp_proxy_client.GetProxyAddress()
    }
    result_callback(&rtp_command_result{
        rtpproxy_address : rtpproxy_address,
        rtpproxy_port : t1[0],
        family : family,
    })
}

func (self *_rtpps_side) _on_sdp_change(sdp_body sippy_types.MsgBody, result_callback func(sippy_types.MsgBody)) {
    sects := []*sippy_sdp.SdpMediaDescription{}
    for _, sect := range sdp_body.GetParsedBody().GetSections() {
        switch strings.ToLower(sect.GetMHeader().GetTransport()) {
        case "udp":
        case "udptl":
        case "rtp/avp":
        default:
            sects = append(sects, sect)
        }
    }
    if len(sects) == 0 {
        sdp_body.SetNeedsUpdate(false)
        result_callback(sdp_body)
        return
    }
    formats := sects[0].GetMHeader().GetFormats()
    self.codecs = strings.Join(formats, ",")
    for i, sect := range sects {
        options := ""
        if sect.GetCHeader().GetAType() == "IP6" {
            options = "6"
        }
        self.update(sect.GetCHeader().GetAddr(), sect.GetMHeader().GetPort(),
              func (res *rtp_command_result) { self._sdp_change_finish(res, sdp_body, sect, sects, result_callback) },
              options, i, sect.GetCHeader().GetAType())
    }
}

func (self *_rtpps_side) _sdp_change_finish(address_port *rtp_command_result, sdp_body sippy_types.MsgBody, sect *sippy_sdp.SdpMediaDescription, sects []*sippy_sdp.SdpMediaDescription, result_callback func(sippy_types.MsgBody)) {
    sect.SetNeedsUpdate(false)
    if address_port != nil {
        sect.GetCHeader().SetAType(address_port.family)
        sect.GetCHeader().SetAddr(address_port.rtpproxy_address)
        if sect.GetMHeader().GetPort() != "0" {
            sect.GetMHeader().SetPort(address_port.rtpproxy_port)
        }
    }
    for _, s := range sects {
        if s.NeedsUpdate() {
            sdp_body.GetParsedBody().SetOHeader(self.origin)
            if self.owner.insert_nortpp {
                sdp_body.GetParsedBody().AppendAHeader("nortpproxy=yes")
            }
            sdp_body.SetNeedsUpdate(false)
            result_callback(sdp_body)
            return
        }
    }
}


