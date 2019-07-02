// Copyright (c) 2019 Sippy Software, Inc. All rights reserved.
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
    "sippy/sdp"
    "sippy/types"
)

type SdpSession struct {
    last_origin *sippy_sdp.SdpOrigin
    origin      *sippy_sdp.SdpOrigin
}

func NewSdpSession() *SdpSession {
    return &SdpSession{
        origin      : sippy_sdp.NewSdpOrigin(),
    }
}

func (self *SdpSession) FixupVersion(event sippy_types.CCEvent) error {
    sdp_body := event.GetBody()
    if sdp_body == nil {
        return nil // no SDP so there is nothing to do
    }
    parsed_body, err := sdp_body.GetParsedBody()
    if err != nil {
        return err
    }
    new_origin := parsed_body.GetOHeader().GetCopy()
    if self.last_origin != nil {
        if self.last_origin.GetSessionId() != new_origin.GetSessionId() ||
                self.last_origin.GetVersion() != new_origin.GetVersion() {
            // Please be aware that this code is not RFC-4566 compliant in case when
            // the session is reused for hunting through several call legs. In that
            // scenario the outgoing SDP should be compared with the previously sent
            // one.
            self.origin.IncVersion()
        }
    }
    self.last_origin = new_origin
    parsed_body.SetOHeader(self.origin.GetCopy())
    sdp_body.SetNeedsUpdate(false)
    return nil
}
