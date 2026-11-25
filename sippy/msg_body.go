// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2015 Andrii Pylypenko. All rights reserved.
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
	"mime"
	"unicode/utf8"

	"github.com/sippy/go-b2bua/sippy/exceptions"
	"github.com/sippy/go-b2bua/sippy/fmt"
	"github.com/sippy/go-b2bua/sippy/net"
	"github.com/sippy/go-b2bua/sippy/types"
)

type msgBody struct {
	mtype          string
	sdp            sippy_types.Sdp
	string_content string
	needs_update   bool
}

func NewMsgBody(content, mtype string) *msgBody {
	return &msgBody{
		mtype:          mtype,
		sdp:            nil,
		string_content: content,
		needs_update:   true,
	}
}

// IsSdpBody reports whether the top-level MIME entity is SDP, without
// parsing its contents. Multipart and other opaque bodies are not offers.
func IsSdpBody(body sippy_types.MsgBody) bool {
	return body != nil && isSdpType(body.GetMtype())
}

func isSdpType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/sdp"
}

func (self *msgBody) GetSdp() (sippy_types.Sdp, sippy_types.SipHandlingError) {
	if self.sdp != nil {
		return self.sdp, nil
	}
	if !IsSdpBody(self) {
		return nil, sippy_exceptions.NewSdpParseError("Not an SDP message")
	}
	if !utf8.ValidString(self.string_content) {
		return nil, sippy_exceptions.NewSdpParseError("SDP is not valid UTF-8")
	}
	sdp, err := ParseSdpBody(self.string_content)
	if err != nil {
		return nil, sippy_exceptions.NewSdpParseError(sippy_fmt.Sprintf("error parsing the SDP: %s", err.Error()))
	}
	self.sdp = sdp
	return sdp, nil
}

func (self *msgBody) String() string {
	if self.sdp != nil {
		return self.sdp.String()
	}
	return self.string_content
}

func (self *msgBody) LocalStr(local_hostport *sippy_net.HostPort) string {
	if self.sdp != nil {
		return self.sdp.LocalStr(local_hostport)
	}
	return self.String()
}

func (self *msgBody) GetCopy() sippy_types.MsgBody {
	if self == nil {
		return nil
	}
	var sdp sippy_types.Sdp
	if self.sdp != nil {
		sdp = self.sdp.GetCopy()
	}
	return &msgBody{
		mtype:          self.mtype,
		sdp:            sdp,
		string_content: self.string_content,
		needs_update:   true,
	}
}

func (self *msgBody) GetMtype() string {
	return self.mtype
}

func (self *msgBody) NeedsUpdate() bool {
	return self.needs_update
}

func (self *msgBody) SetNeedsUpdate(v bool) {
	self.needs_update = v
}

func (self *msgBody) AppendAHeader(hdr string) {
	if self.sdp != nil {
		self.sdp.AppendAHeader(hdr)
	} else if IsSdpBody(self) {
		self.string_content += "a=" + hdr + "\r\n"
	}
}
