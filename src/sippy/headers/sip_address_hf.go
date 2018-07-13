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
package sippy_header

import (
    "strings"

    "sippy/conf"
    "sippy/net"
)

type sipAddressHFBody struct {
    Address *SipAddress
}

func (self *sipAddressHFBody) getCopy() *sipAddressHFBody {
    return &sipAddressHFBody{
        Address : self.Address.GetCopy(),
    }
}

type sipAddressHF struct {
    string_body     string
    body            *sipAddressHFBody
}

func newSipAddressHF(addr *SipAddress) *sipAddressHF {
    return &sipAddressHF{
        body    : &sipAddressHFBody{ Address : addr },
    }
}

func createSipAddressHFs(body string) []*sipAddressHF {
    addresses := []string{}
    pidx := 0
    for {
        idx := strings.IndexRune(body[pidx:], ',')
        if idx == -1 {
            addresses = append(addresses, body[pidx:])
            break
        }
        onum, cnum, qnum := 0, 0, 0
        for _, r := range body[:idx] {
            switch r {
            case '<': onum++
            case '>': cnum++
            case '"': qnum ++
            }
        }
        if (onum == 0 && cnum == 0 && qnum == 0) || (onum > 0 &&
          onum == cnum && (qnum % 2 == 0)) {
            addresses = append(addresses, body[:idx])
            body = body[idx + 1:]
            pidx = 0
        } else {
            pidx = idx + 1
        }
    }
    retval := make([]*sipAddressHF, len(addresses))
    for i, address := range addresses {
        retval[i] = &sipAddressHF{ string_body : address }
    }
    return retval
}

func (self *sipAddressHF) parse(config sippy_conf.Config) error {
    addr, err := ParseSipAddress(self.string_body, false /* relaxedparser */, config)
    if err != nil {
        return err
    }
    self.body = &sipAddressHFBody{
        Address     : addr,
    }
    return nil
}

func (self *sipAddressHF) getCopy() *sipAddressHF {
    cself := *self
    if self.body != nil {
        cself.body = self.body.getCopy()
    }
    return &cself
}

func (self *sipAddressHF) GetBody(config sippy_conf.Config) (*SipAddress, error) {
    if self.body == nil {
        if err := self.parse(config); err != nil {
            return nil, err
        }
    }
    return self.body.Address, nil
}

func (self *sipAddressHF) StringBody() string {
    return self.LocalStringBody(nil)
}

func (self *sipAddressHF) LocalStringBody(hostport *sippy_net.HostPort) string {
    if self.body != nil {
        return self.body.Address.LocalStr(hostport)
    }
    return self.string_body
}
