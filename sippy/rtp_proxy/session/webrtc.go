// Copyright (c) 2024 Sippy Software, Inc. All rights reserved.
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

// SetCallerWebRTC configures the session for the webrtc2sip topology
// (Python: Rtp_proxy_session_webrtc2sip): the caller is the WebRTC
// (ICE-lite/DTLS) side, the callee the plain RTP side.
//
//   - caller.deice = true, caller.gateway_dtls = "dtls"
//   - callee.gateway_dtls = "rtp"
//
// Passing false resets both sides to the ordinary RTP defaults
// (gateway_dtls off, deice off). Setting both caller and callee WebRTC is
// not a supported configuration; the last call wins per side.
func (self *Rtp_proxy_session) SetCallerWebRTC(v bool) {
    if v {
        self.caller.deice = true
        self.caller.gateway_dtls = "dtls"
        self.callee.gateway_dtls = "rtp"
    } else {
        self.caller.deice = false
        self.caller.gateway_dtls = "pass"
        self.callee.gateway_dtls = "pass"
    }
}

// SetCalleeWebRTC configures the session for the sip2webrtc topology
// (Python: Rtp_proxy_session_sip2webrtc): the callee is the WebRTC
// (ICE-lite/DTLS) side, the caller the plain RTP side.
//
//   - callee.deice = true, callee.gateway_dtls = "dtls"
//   - caller.gateway_dtls = "rtp"
//
// Passing false resets both sides to the ordinary RTP defaults.
func (self *Rtp_proxy_session) SetCalleeWebRTC(v bool) {
    if v {
        self.callee.deice = true
        self.callee.gateway_dtls = "dtls"
        self.caller.gateway_dtls = "rtp"
    } else {
        self.callee.deice = false
        self.callee.gateway_dtls = "pass"
        self.caller.gateway_dtls = "pass"
    }
}
