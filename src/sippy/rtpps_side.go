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
    "sync"

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
    repacketize     int
    origin_lock     sync.Mutex
    oh_remote       *sippy_sdp.SdpOrigin
    after_sdp_change func(sippy_types.RtpProxyUpdateResult)
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
    command := fmt.Sprintf("P%d %s-%d %s %s %s %s", times, self.owner.call_id, index, prompt_name, self.codecs, self.owner.from_tag, self.owner.to_tag)
    self.owner.rtp_proxy_client.SendCommand(command, func(r string) { self.owner.command_result(r, result_callback) }, self.owner.session_lock)
}

func (self *_rtpps_side) update(remote_ip string, remote_port string, result_callback func(*rtpproxy_update_result), options/*= ""*/ string, index /*= 0*/int, atype /*= "IP4"*/string) {
    command := "U"
    self.owner.max_index = int(math.Max(float64(self.owner.max_index), float64(index)))
    if self.owner.rtp_proxy_client.SBindSupported() {
        if self.raddress != nil {
            //if self.owner.rtp_proxy_client.IsLocal() && atype == "IP4" {
            //    options += fmt.Sprintf("L%s", self.laddress)
            //} else if ! self.owner.rtp_proxy_client.IsLocal() {
            //    options += fmt.Sprintf("R%s", self.raddress.Host.String())
            //}
            options += "R" + self.raddress.Host.String()
        } else if self.laddress != "" && self.owner.rtp_proxy_client.IsLocal() {
            options += "L" + self.laddress
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
    self.owner.rtp_proxy_client.SendCommand(command, func(r string) { self.update_result(r, remote_ip, atype, result_callback) }, self.owner.session_lock)
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
        rtpproxy_address = self.owner.rtp_proxy_client.GetProxyAddress()
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
    parsed_body, err := sdp_body.GetParsedBody()
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
        options = fmt.Sprintf("z%d", self.repacketize)
    }
    for i, sect := range sects {
        sect_options := options
        if sect.GetCHeader().GetAType() == "IP6" {
            sect_options = "6" + options
        }
        self.update(sect.GetCHeader().GetAddr(), sect.GetMHeader().GetPort(),
              func (res *rtpproxy_update_result) { self._sdp_change_finish(res, sdp_body, parsed_body, sect, sects, result_callback) },
              sect_options, i, sect.GetCHeader().GetAType())
    }
    return nil
}

func (self *_rtpps_side) _sdp_change_finish(cb_args *rtpproxy_update_result, sdp_body sippy_types.MsgBody, parsed_body sippy_types.ParsedMsgBody, sect *sippy_sdp.SdpMediaDescription, sects []*sippy_sdp.SdpMediaDescription, result_callback func(sippy_types.MsgBody)) {
    sect.SetNeedsUpdate(false)
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
            sect.RemoveAHeader("sendonly")
            sect.AddHeader("a", "sendonly")
        }
        if self.repacketize > 0 {
            sect.RemoveAHeader("ptime:")
            sect.AddHeader("a", fmt.Sprintf("ptime:%d", self.repacketize))
        }
    }
    num := 0
    for _, s := range sects {
        if s.NeedsUpdate() {
            num++
        }
    }
    if num == 0 {
        self.origin_lock.Lock()
        if self.oh_remote != nil {
            if parsed_body.GetOHeader() != nil {
                if self.oh_remote.GetSessionId() != parsed_body.GetOHeader().GetSessionId() ||
                        self.oh_remote.GetVersion() != parsed_body.GetOHeader().GetVersion() {
                    // Please be aware that this code is not RFC-4566 compliant in case when
                    // the session is reused for hunting through several call legs. In that
                    // scenario the outgoing SDP should be compared with the previously sent
                    // one.
                    self.origin.IncVersion()
                }
            }
        }
        self.oh_remote = parsed_body.GetOHeader().GetCopy()
        parsed_body.SetOHeader(self.origin.GetCopy())
        self.origin_lock.Unlock()
        if self.owner.insert_nortpp {
            parsed_body.AppendAHeader("nortpproxy=yes")
        }
        sdp_body.SetNeedsUpdate(false)
        result_callback(sdp_body)
    }
}
