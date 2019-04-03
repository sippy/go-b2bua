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
    "crypto/rand"
    "errors"
    "fmt"
    "math/big"
    "runtime"
    "sync"

    "sippy/conf"
    "sippy/net"
    "sippy/sdp"
    "sippy/types"
)

type Rtp_proxy_session struct {
    call_id                 string
    from_tag                string
    to_tag                  string
    _rtp_proxy_client       sippy_types.RtpProxyClient
    max_index               int
    l4r                     *local4remote
    notify_socket           string
    notify_tag              string
    insert_nortpp           bool
    caller                  _rtpps_side
    callee                  _rtpps_side
    session_lock            sync.Locker
    config                  sippy_conf.Config
    inflight_lock           sync.Mutex
    inflight_cmd            *rtpp_cmd
    rtpp_wi                 chan *rtpp_cmd
}

type rtpproxy_update_result struct {
    rtpproxy_address    string
    rtpproxy_port       string
    family              string
    sendonly            bool
}

type rtpp_cmd struct {
    cmd         string
    cb          func(string)
    rtp_proxy_client sippy_types.RtpProxyClient
}

func (self *rtpproxy_update_result) Address() string {
    return self.rtpproxy_address
}

func NewRtp_proxy_session(config sippy_conf.Config, rtp_proxy_clients []sippy_types.RtpProxyClient, call_id, from_tag, to_tag, notify_socket, notify_tag string, session_lock sync.Locker, callee_origin *sippy_sdp.SdpOrigin) (*Rtp_proxy_session, error) {
    self := &Rtp_proxy_session{
        notify_socket   : notify_socket,
        notify_tag      : notify_tag,
        call_id         : call_id,
        from_tag        : from_tag,
        to_tag          : to_tag,
        insert_nortpp   : false,
        max_index       : -1,
        session_lock    : session_lock,
        config          : config,
        rtpp_wi         : make(chan *rtpp_cmd, 50),
    }
    self.caller.otherside = &self.callee
    self.callee.otherside = &self.caller
    self.caller.owner = self
    self.callee.owner = self
    self.caller.session_exists = false
    self.callee.session_exists = false
    self.caller.origin = sippy_sdp.NewSdpOrigin()
    if callee_origin != nil {
        // New session means new RTP port so the SDP is now different and the SDP
        // version must be increased.
        callee_origin.IncVersion()
        self.callee.origin = callee_origin
    } else {
        self.callee.origin = sippy_sdp.NewSdpOrigin()
    }
    online_clients := []sippy_types.RtpProxyClient{}
    for _, cl := range rtp_proxy_clients {
        if cl.IsOnline() {
            online_clients = append(online_clients, cl)
        }
    }
    n := len(online_clients)
    if n == 0 {
        return nil, fmt.Errorf("No online RTP proxy client has been found")
    }
    if idx, err := rand.Int(rand.Reader, big.NewInt(int64(n))); err != nil {
        self._rtp_proxy_client = online_clients[0]
    } else {
        self._rtp_proxy_client = online_clients[idx.Int64()]
    }
    if self.call_id == "" {
        buf := make([]byte, 16)
        rand.Read(buf)
        self.call_id = fmt.Sprintf("%x", buf)
    }
    if from_tag == "" {
        buf := make([]byte, 16)
        rand.Read(buf)
        self.from_tag = fmt.Sprintf("%x", buf)
    }
    if to_tag == "" {
        buf := make([]byte, 16)
        rand.Read(buf)
        self.to_tag = fmt.Sprintf("%x", buf)
    }
    self.caller.from_tag = self.from_tag
    self.caller.to_tag = self.to_tag
    self.callee.to_tag = self.from_tag
    self.callee.from_tag = self.to_tag
    runtime.SetFinalizer(self, rtp_proxy_session_destructor)
    return self, nil
}
/*
    def version(self, result_callback):
        self.send_command("V", self.version_result, result_callback)

    def version_result(self, result, result_callback):
        result_callback(result)
*/
func (self *Rtp_proxy_session) PlayCaller(prompt_name string, times int/*= 1*/, result_callback func(string)/*= nil*/, index int /*= 0*/) {
    self.caller._play(prompt_name, times, result_callback, index)
}

func (self *Rtp_proxy_session) send_command(cmd string, cb func(string)) {
    if rtp_proxy_client := self._rtp_proxy_client; rtp_proxy_client != nil {
        self.inflight_lock.Lock()
        defer self.inflight_lock.Unlock()
        new_cmd := &rtpp_cmd{ cmd, cb, rtp_proxy_client }
        if self.inflight_cmd == nil {
            self.inflight_cmd = new_cmd
            rtp_proxy_client.SendCommand(cmd, self.cmd_done)
        } else {
            self.rtpp_wi <- new_cmd
        }
    }
}

func (self *Rtp_proxy_session) cmd_done(res string) {
    self.inflight_lock.Lock()
    done_cmd := self.inflight_cmd
    select {
        case self.inflight_cmd = <-self.rtpp_wi:
            self.inflight_cmd.rtp_proxy_client.SendCommand(self.inflight_cmd.cmd, self.cmd_done)
        default:
            self.inflight_cmd = nil
    }
    self.inflight_lock.Unlock()
    if done_cmd != nil && done_cmd.cb != nil {
        self.session_lock.Lock()
        done_cmd.cb(res)
        self.session_lock.Unlock()
    }
}

func (self *Rtp_proxy_session) StopPlayCaller(result_callback func(string)/*= nil*/, index int/*= 0*/) {
    self.caller._stop_play(result_callback, index)
}

func (self *Rtp_proxy_session) StartRecording(rname/*= nil*/ string, result_callback func(string)/*= nil*/, index int/*= 0*/) {
    if ! self.caller.session_exists {
        self.caller.update("0.0.0.0", "0", func(*rtpproxy_update_result) { self._start_recording(rname, result_callback, index) }, "", index, "IP4")
        return
    }
    self._start_recording(rname, result_callback, index)
}

func (self *Rtp_proxy_session) _start_recording(rname string, result_callback func(string), index int) {
    if rname == "" {
        command := fmt.Sprintf("R %s-%d %s %s", self.call_id, index, self.from_tag, self.to_tag)
        self.send_command(command, func (r string) { self.command_result(r, result_callback) })
        return
    }
    command := fmt.Sprintf("C %s-%d %s.a %s %s", self.call_id, index, rname, self.from_tag, self.to_tag)
    self.send_command(command, func(string) { self._start_recording1(rname, result_callback, index) })
}

func (self *Rtp_proxy_session) _start_recording1(rname string, result_callback func(string), index int) {
    command := fmt.Sprintf("C %s-%d %s.o %s %s", self.call_id, index, rname, self.to_tag, self.from_tag)
    self.send_command(command, func (r string) { self.command_result(r, result_callback) })
}

func (self *Rtp_proxy_session) command_result(result string, result_callback func(string)) {
    //print "%s.command_result(%s)" % (id(self), result)
    if result_callback != nil {
        result_callback(result)
    }
}

func (self *Rtp_proxy_session) Delete() {
    if self._rtp_proxy_client == nil {
        return
    }
    for self.max_index >= 0 {
        command := fmt.Sprintf("D %s-%d %s %s", self.call_id, self.max_index, self.from_tag, self.to_tag)
        self.send_command(command, nil)
        self.max_index--
    }
    self._rtp_proxy_client = nil
}

func (self *Rtp_proxy_session) OnCallerSdpChange(sdp_body sippy_types.MsgBody, result_callback func(sippy_types.MsgBody)) error {
    return self.caller._on_sdp_change(sdp_body, result_callback)
}

func (self *Rtp_proxy_session) OnCalleeSdpChange(sdp_body sippy_types.MsgBody, result_callback func(sippy_types.MsgBody)) error {
    return self.callee._on_sdp_change(sdp_body, result_callback)
}

func rtp_proxy_session_destructor(self *Rtp_proxy_session) {
    self.Delete()
}

func (self *Rtp_proxy_session) CallerSessionExists() bool { return self.caller.session_exists }

func (self *Rtp_proxy_session) SetCallerLaddress(addr string) {
    self.caller.laddress = addr
}

func (self *Rtp_proxy_session) SetCallerRaddress(addr *sippy_net.HostPort) {
    self.caller.raddress = addr
}

func (self *Rtp_proxy_session) SetCalleeLaddress(addr string) {
    self.callee.laddress = addr
}

func (self *Rtp_proxy_session) SetCalleeRaddress(addr *sippy_net.HostPort) {
    self.callee.raddress = addr
}

func (self *Rtp_proxy_session) SetInsertNortpp(v bool) {
    self.insert_nortpp = v
}

func (self *Rtp_proxy_session) SetAfterCallerSdpChange(cb func(sippy_types.RtpProxyUpdateResult)) {
    self.caller.after_sdp_change = cb
}

func (self *Rtp_proxy_session) CalleeOrigin() *sippy_sdp.SdpOrigin {
    if self == nil {
        return nil
    }
    return self.callee.origin
}

func (self Rtp_proxy_session) SBindSupported() (bool, error) {
    rtp_proxy_client := self._rtp_proxy_client
    if rtp_proxy_client == nil {
        return true, errors.New("the session already deleted")
    }
    return rtp_proxy_client.SBindSupported(), nil
}

func (self Rtp_proxy_session) IsLocal() (bool, error) {
    rtp_proxy_client := self._rtp_proxy_client
    if rtp_proxy_client == nil {
        return true, errors.New("the session already deleted")
    }
    return rtp_proxy_client.IsLocal(), nil
}

func (self Rtp_proxy_session) TNotSupported() (bool, error) {
    rtp_proxy_client := self._rtp_proxy_client
    if rtp_proxy_client == nil {
        return true, errors.New("the session already deleted")
    }
    return rtp_proxy_client.TNotSupported(), nil
}

func (self Rtp_proxy_session) GetProxyAddress() (string, error) {
    rtp_proxy_client := self._rtp_proxy_client
    if rtp_proxy_client == nil {
        return "", errors.New("the session already deleted")
    }
    return rtp_proxy_client.GetProxyAddress(), nil
}
