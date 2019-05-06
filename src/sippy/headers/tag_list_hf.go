// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2019 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2019 Andrii Pylypenko. All rights reserved.
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
package sippy_header

import (
    "strings"
)

type tagListBody struct {
    Tags    []string
}

func newTagListBody(body string) *tagListBody {
    tags := make([]string, 0)
    for _, s := range strings.Split(body, ",") {
        tags = append(tags, strings.TrimSpace(s))
    }
    return &tagListBody{
        Tags    : tags,
    }
}

func (self *tagListBody) String() string {
    return strings.Join(self.Tags, ", ")
}

func (self *tagListBody) HasTag(tag string) bool {
    for _, stored_tag := range self.Tags {
        if stored_tag == tag {
            return true
        }
    }
    return false
}

type tagListHF struct {
    string_body string
    body        *tagListBody
}

func createTagListHF(body string) *tagListHF {
    return &tagListHF{
        string_body : body,
    }
}

func (self *tagListHF) GetBody() *tagListBody {
    if self.body == nil {
        self.body = newTagListBody(self.string_body)
    }
    return self.body
}

func (self *tagListHF) getCopy() *tagListHF {
    tmp := *self
    if self.body != nil {
        body := *self.body
        tmp.body = &body
    }
    return &tmp
}

func (self *tagListHF) StringBody() string {
    if self.body != nil {
        return self.body.String()
    }
    return self.string_body
}

func (self *tagListHF) HasTag(tag string) bool {
    body := self.GetBody()
    return body.HasTag(tag)
}
