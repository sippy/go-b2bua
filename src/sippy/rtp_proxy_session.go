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
    "strings"
    "strconv"

    "sippy/conf"
    "sippy/types"
)

type Rtp_proxy_session struct {
    caller_session_exists   bool
    call_id                 string
    from_tag                string
    to_tag                  string
    rtp_proxy_client        sippy_types.RtpProxyClient
    max_index               int
    caller_raddress         *sippy_conf.HostPort
    callee_raddress         *sippy_conf.HostPort
    l4r                     *local4remote
    callee_session_exists   bool
    notify_socket           string
    notify_tag              string
    caller_codecs           string
}

type rtp_command_result struct {
    rtpproxy_address    string
    rtpproxy_port       string
    family              string
}

/*
from SdpOrigin import SdpOrigin

from hashlib import md5
from random import random
from time import time
from datetime import datetime
from traceback import print_exc
from thread import get_ident
import sys

from twisted.internet import reactor

class Rtp_proxy_session(object):
    rtp_proxy_client = nil
    call_id = nil
    from_tag = nil
    to_tag = nil
    caller_session_exists = False
    caller_codecs = nil
    caller_raddress = nil
    callee_session_exists = False
    callee_codecs = nil
    callee_raddress = nil
    max_index = -1
    origin = nil
    notify_socket = nil
    notify_tag = nil
    global_config = nil
    my_ident = nil

    def __init__(self, global_config, call_id = nil, from_tag = nil, to_tag = nil,
      notify_socket = nil, notify_tag = nil):
        self.global_config = global_config
        self.my_ident = get_ident()
        if global_config.has_key("_rtp_proxy_clients"):
            rtp_proxy_clients = [x for x in global_config["_rtp_proxy_clients"] if x.online]
            n = len(rtp_proxy_clients)
            if n == 0:
                raise Exception("No online RTP proxy client has been found")
            self.rtp_proxy_client = rtp_proxy_clients[int(random() * n)]
        else:
            self.rtp_proxy_client = global_config["rtp_proxy_client"]
            if not self.rtp_proxy_client.online:
                raise Exception("No online RTP proxy client has been found")
        if call_id != nil:
            self.call_id = call_id
        else:
            self.call_id = md5(str(random()) + str(time())).hexdigest()
        if from_tag != nil:
            self.from_tag = from_tag
        else:
            self.from_tag = md5(str(random()) + str(time())).hexdigest()
        if to_tag != nil:
            self.to_tag = to_tag
        else:
            self.to_tag = md5(str(random()) + str(time())).hexdigest()
        self.origin = SdpOrigin()
        self.notify_socket = notify_socket
        self.notify_tag = notify_tag

    def version(self, result_callback):
        self.rtp_proxy_client.SendCommand("V", self.version_result, result_callback)

    def version_result(self, result, result_callback):
        result_callback(result)
*/
func (self *Rtp_proxy_session) PlayCaller(prompt_name string, times int/*= 1*/, result_callback func(string)/*= nil*/, index int /*= 0*/) {
    if ! self.caller_session_exists {
        return
    }
    if ! self.callee_session_exists {
        self.update_callee("0.0.0.0", "0", func(*rtp_command_result) { self._play_caller(prompt_name, times, result_callback, index) }, "", index, "IP4")
        return
    }
    self._play_caller(prompt_name, times, result_callback, index)
}

func (self *Rtp_proxy_session) _play_caller(prompt_name string, times int, result_callback func(string), index int) {
    command := fmt.Sprintf("P%d %s-%d %s %s %s %s", times, self.call_id, index, prompt_name, self.caller_codecs, self.from_tag, self.to_tag)
    self.rtp_proxy_client.SendCommand(command, func(r string) { self.command_result(r, result_callback) })
}
/*
    def play_callee(self, prompt_name, times = 1, result_callback = nil, index = 0):
        if not self.callee_session_exists:
            return
        if not self.caller_session_exists:
            self.update_caller("0.0.0.0", 0, self._play_callee, "", index, "IP4", prompt_name, times, result_callback, index)
            return
        self._play_callee(nil, prompt_name, times, result_callback, index)

    def _play_callee(self, result, prompt_name, times, result_callback, index):
        command = "P%d %s %s %s %s %s" % (times, "%s-%d" % (self.call_id, index), prompt_name, self.callee_codecs, self.to_tag, self.from_tag)
        self.rtp_proxy_client.SendCommand(command, self.command_result, result_callback)
*/
func (self *Rtp_proxy_session) StopPlayCaller(result_callback func(string)/*= nil*/, index int/*= 0*/) {
    if ! self.caller_session_exists {
        return
    }
    command := fmt.Sprintf("S %s-%d %s %s", self.call_id, index, self.from_tag, self.to_tag)
    self.rtp_proxy_client.SendCommand(command, func(r string) { self.command_result(r, result_callback) })
}
/*
    def stop_play_callee(self, result_callback = nil, index = 0):
        if not self.caller_session_exists:
            return
        command = "S %s %s %s" % ("%s-%d" % (self.call_id, index), self.to_tag, self.from_tag)
        self.rtp_proxy_client.SendCommand(command, self.command_result, result_callback)

    def copy_caller(self, remote_ip, remote_port, result_callback = nil, index = 0):
        if not self.caller_session_exists:
            self.update_caller("0.0.0.0", 0, self._copy_caller, "", index, "IP4", remote_ip, remote_port, result_callback, index)
            return
        self._copy_caller(nil, remote_ip, remote_port, result_callback, index)

    def _copy_caller(self, result, remote_ip, remote_port, result_callback = nil, index = 0):
        command = "C %s udp:%s:%d %s %s" % ("%s-%d" % (self.call_id, index), remote_ip, remote_port, self.from_tag, self.to_tag)
        self.rtp_proxy_client.SendCommand(command, self.command_result, result_callback)

    def copy_callee(self, remote_ip, remote_port, result_callback = nil, index = 0):
        if not self.callee_session_exists:
            self.update_callee("0.0.0.0", 0, self._copy_callee, "", index, "IP4", remote_ip, remote_port, result_callback, index)
            return
        self._copy_callee(nil, remote_ip, remote_port, result_callback, index)

    def _copy_callee(self, result, remote_ip, remote_port, result_callback = nil, index = 0):
        command = "C %s udp:%s:%d %s %s" % ("%s-%d" % (self.call_id, index), remote_ip, remote_port, self.to_tag, self.from_tag)
        self.rtp_proxy_client.SendCommand(command, self.command_result, result_callback)
*/
func (self *Rtp_proxy_session) StartRecording(rname/*= nil*/ string, result_callback func(string)/*= nil*/, index int/*= 0*/) {
    if ! self.caller_session_exists {
        self.update_caller("0.0.0.0", "0", func(*rtp_command_result) { self._start_recording(rname, result_callback, index) }, "", index, "IP4")
        return
    }
    self._start_recording(rname, result_callback, index)
}

func (self *Rtp_proxy_session) _start_recording(rname string, result_callback func(string), index int) {
    if rname == "" {
        command := fmt.Sprintf("R %s-%d %s %s", self.call_id, index, self.from_tag, self.to_tag)
        self.rtp_proxy_client.SendCommand(command, func (r string) { self.command_result(r, result_callback) })
        return
    }
    command := fmt.Sprintf("C %s-%d %s.a %s %s", self.call_id, index, rname, self.from_tag, self.to_tag)
    self.rtp_proxy_client.SendCommand(command, func(string) { self._start_recording1(rname, result_callback, index) })
}

func (self *Rtp_proxy_session) _start_recording1(rname string, result_callback func(string), index int) {
    command := fmt.Sprintf("C %s-%d %s.o %s %s", self.call_id, index, rname, self.to_tag, self.from_tag)
    self.rtp_proxy_client.SendCommand(command, func (r string) { self.command_result(r, result_callback) })
}

func (self *Rtp_proxy_session) command_result(result string, result_callback func(string)) {
    //print "%s.command_result(%s)" % (id(self), result)
    if result_callback != nil {
        result_callback(result)
    }
}

func (self *Rtp_proxy_session) update_caller(remote_ip string, remote_port string, result_callback func(*rtp_command_result), options/*= ""*/ string, index /*= 0*/int, atype /*= "IP4"*/string /*, *callback_parameters*/) {
    command := "U"
    self.max_index = int(math.Max(float64(self.max_index), float64(index)))
    if self.rtp_proxy_client.SBindSupported() && self.caller_raddress != nil && atype == "IP4" {
        if self.rtp_proxy_client.IsLocal() {
            options += fmt.Sprintf("L%s", self.l4r.getServer(self.caller_raddress, false).uopts.laddress.Host.String())
        } else {
            options += fmt.Sprintf("R%s", self.caller_raddress.Host.String())
        }
    }
    command += options
    if self.callee_session_exists {
        command += fmt.Sprintf(" %s-%d %s %s %s %s", self.call_id, index, remote_ip, remote_port, self.from_tag, self.to_tag)
    } else {
        command += fmt.Sprintf(" %s-%d %s %s %s", self.call_id, index, remote_ip, remote_port, self.from_tag)
    }
    if self.notify_socket != "" && index == 0 && self.rtp_proxy_client.TNotSupported() {
        command += fmt.Sprintf(" %s %s", self.notify_socket, self.notify_tag)
    }
    self.rtp_proxy_client.SendCommand(command, func(r string) { self.update_result(r, result_callback, "caller") })
}

func (self *Rtp_proxy_session) update_callee(remote_ip string, remote_port string, result_callback func(*rtp_command_result), options string/*= ""*/, index int/*= 0*/, atype string/*= "IP4"*/) {
    command := "U"
    self.max_index = int(math.Max(float64(self.max_index), float64(index)))
    if self.rtp_proxy_client.SBindSupported() && self.callee_raddress != nil && atype == "IP4" {
        if self.rtp_proxy_client.IsLocal() {
            options += fmt.Sprintf("L%s", self.l4r.getServer(self.callee_raddress, false).uopts.laddress.Host.String())
        } else {
            options += fmt.Sprintf("R%s", self.callee_raddress.Host.String())
        }
    }
    command += options
    if self.caller_session_exists {
        command += fmt.Sprintf(" %s-%d %s %d %s %s", self.call_id, index, remote_ip, remote_port, self.to_tag, self.from_tag)
    } else {
        command += fmt.Sprintf(" %s-%d %s %d %s", self.call_id, index, remote_ip, remote_port, self.to_tag)
    }
    if self.notify_socket != "" && index == 0 && self.rtp_proxy_client.TNotSupported() {
        command += fmt.Sprintf(" %s %s", self.notify_socket, self.notify_tag)
    }
    self.rtp_proxy_client.SendCommand(command, func(r string) { self.update_result(r, result_callback, "callee") })
}

func (self *Rtp_proxy_session) update_result(result string, result_callback func(*rtp_command_result), face string) {
    //print "%s.update_result(%s)" % (id(self), result)
    //result_callback, face, callback_parameters = args
    if face == "caller" {
        self.caller_session_exists = true
    } else {
        self.callee_session_exists = true
    }
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
        rtpproxy_address = self.rtp_proxy_client.GetProxyAddress()
    }
    result_callback(&rtp_command_result{
        rtpproxy_address : rtpproxy_address,
        rtpproxy_port : t1[0],
        family : family,
    })
}

/*
    def delete(self):
        if self.rtp_proxy_client == nil:
            return
        while self.max_index >= 0:
            command = "D %s %s %s" % ("%s-%d" % (self.call_id, self.max_index), self.from_tag, self.to_tag)
            self.rtp_proxy_client.SendCommand(command)
            self.max_index -= 1
        self.rtp_proxy_client = nil

    def on_caller_sdp_change(self, sdp_body, result_callback):
        self.on_xxx_sdp_change(self.update_caller, sdp_body, result_callback)

    def on_callee_sdp_change(self, sdp_body, result_callback):
        self.on_xxx_sdp_change(self.update_callee, sdp_body, result_callback)

    def on_xxx_sdp_change(self, update_xxx, sdp_body, result_callback):
        sects = []
        try:
            sdp_body.parse()
        except Exception, exception:
            print datetime.now(), "can't parse SDP body: %s:" % str(exception)
            print "-" * 70
            print_exc(file = sys.stdout)
            print "-" * 70
            print sdp_body.content
            print "-" * 70
            sys.stdout.flush()
            return
        for i in range(0, len(sdp_body.content.sections)):
            sect = sdp_body.content.sections[i]
            if sect.m_header.transport.lower() not in ("udp", "udptl", "rtp/avp"):
                continue
            sects.append(sect)
        if len(sects) == 0:
            sdp_body.needs_update = False
            result_callback(sdp_body)
            return
        formats = sects[0].m_header.formats
        if update_xxx == self.update_caller:
            if len(formats) > 1:
                self.caller_codecs = reduce(lambda x, y: str(x) + "," + str(y), formats)
            else:
                self.caller_codecs = str(formats[0])
        else:
            if len(formats) > 1:
                self.callee_codecs = reduce(lambda x, y: str(x) + "," + str(y), formats)
            else:
                self.callee_codecs = str(formats[0])
        for sect in sects:
            options = ""
            if sect.c_header.atype == "IP6":
                options = "6"
            update_xxx(sect.c_header.addr, sect.m_header.port, self.xxx_sdp_change_finish, options, \
              sects.index(sect), sect.c_header.atype, sdp_body, sect, sects, result_callback)
        return

    def xxx_sdp_change_finish(self, address_port, sdp_body, sect, sects, result_callback):
        sect.needs_update = False
        if address_port != nil:
            sect.c_header.atype = address_port[2]
            sect.c_header.addr = address_port[0]
            if sect.m_header.port != 0:
                sect.m_header.port = address_port[1]
        if len([x for x in sects if x.needs_update]) == 0:
            sdp_body.content.o_header = self.origin
            sdp_body.needs_update = False
            result_callback(sdp_body)

    def __del__(self):
        if self.my_ident != get_ident():
            #print "Rtp_proxy_session.__del__() from wrong thread, re-routing"
            reactor.callFromThread(self.delete)
        else:
            self.delete()
*/

func (self *Rtp_proxy_session) CallerSessionExists() bool { return self.caller_session_exists }
