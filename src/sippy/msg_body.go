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
    "strings"

    "sippy/conf"
    "sippy/sdp"
    "sippy/types"
)

type msgBody struct {
    mtype                   string
    parsed_body             sippy_types.ParsedMsgBody
    string_content          string
    needs_update    bool
}

func NewMsgBody(content, mtype string) *msgBody {
    return &msgBody{
        mtype                   : mtype,
        parsed_body             : nil,
        string_content          : content,
        needs_update            : true,
    }
}

type genericMsgBody struct {
    body string
}

func newGenericMsgBody(body string) *genericMsgBody {
    return &genericMsgBody{ body }
}

func (self *genericMsgBody) String() string {
    return self.body
}

func (self *genericMsgBody) LocalStr(*sippy_conf.HostPort) string {
    return self.body
}

func (self *genericMsgBody) GetCopy() sippy_types.ParsedMsgBody {
    return &genericMsgBody{ self.body }
}

func (self *genericMsgBody) SetCHeaderAddr(string) {
    // NO OP
}

func (self *genericMsgBody) GetSections() []sippy_types.SdpMediaDescription {
    return make([]sippy_types.SdpMediaDescription, 0)
}

func (self *genericMsgBody) SetSections([]sippy_types.SdpMediaDescription) {
    // NO OP
}

func (self *genericMsgBody) RemoveSection(int) {
    // NO OP
}

func (self *genericMsgBody) SetOHeader(*sippy_sdp.SdpOrigin) {
    // NO OP
}

func (self *msgBody) GetParsedBody() sippy_types.ParsedMsgBody {
    if self.parsed_body == nil {
        self.parse()
    }
    return self.parsed_body
}

func (self *msgBody) parse() {
    self.parsed_body = newGenericMsgBody(self.string_content)
    if self.string_content == "" {
        return
    }
    if strings.HasPrefix(self.mtype, "multipart/mixed;") {
        arr := strings.SplitN(self.mtype, ";", 2)
        mtheaders := arr[1]
        var mth_boundary *string = nil
        for _, s := range strings.Split(mtheaders, ";") {
            arr = strings.SplitN(s, "=", 2)
            if arr[0] == "boundary" && len(arr) == 2 {
                mth_boundary = &arr[1]
                break
            }
        }
        if mth_boundary == nil {
            return
        }
        boundary := "--" + *mth_boundary
        for _, subsection := range strings.Split(self.string_content, boundary) {
            subsection = strings.TrimSpace(subsection)
            if subsection == "" { continue }
            boff, bdel := -1, ""
            for _, bdel = range []string{ "\r\n\r\n", "\r\r", "\n\n" } {
                boff = strings.Index(subsection, bdel)
                if boff != -1 {
                    break
                }
            }
            if boff == -1 {
                continue
            }
            mbody := subsection[boff + len(bdel):]
            mtype := ""
            for _, line := range strings.FieldsFunc(subsection[:boff], func(c rune) bool { return c == '\n' || c == '\r' }) {
                tmp := strings.ToLower(strings.TrimSpace(line))
                if strings.HasPrefix(tmp, "content-type:") {
                    arr = strings.SplitN(tmp, ":", 2)
                    mtype = strings.TrimSpace(arr[1])
                }
            }
            if mtype == "" {
                continue
            }
            if mtype == "application/sdp" {
                self.mtype = mtype
                self.string_content = mbody
                break
            }
        }
    }
    if self.mtype == "application/sdp" {
        self.parsed_body = ParseSdpBody(self.string_content)
    }
}

func (self *msgBody) String() string {
    if self.parsed_body != nil {
        self.string_content = self.parsed_body.String()
    }
    return self.string_content
}

func (self *msgBody) LocalStr(local_hostport *sippy_conf.HostPort) string {
    if self.parsed_body != nil {
        return self.parsed_body.LocalStr(local_hostport)
    }
    return self.String()
}

func (self *msgBody) GetCopy() sippy_types.MsgBody {
    if self == nil {
        return nil
    }
    var parsed_body sippy_types.ParsedMsgBody
    if self.parsed_body != nil {
        parsed_body = self.parsed_body.GetCopy()
    } else {
        parsed_body = nil
    }
    return &msgBody{
        mtype                   : self.mtype,
        parsed_body             : parsed_body,
        string_content          : self.string_content,
        needs_update            : self.needs_update,
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
