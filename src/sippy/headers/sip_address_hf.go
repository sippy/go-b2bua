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
)

type sipAddressHF struct {
    Address *sipAddress
}

func NewSipAddressHF(addr *sipAddress) *sipAddressHF {
    return &sipAddressHF{
        Address : addr,
    }
}

func ParseSipAddressHF(body string) ([]*sipAddressHF, error) {
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
        addr, err := ParseSipAddress(address, false /* relaxedparser */)
        if err != nil {
            return nil, err
        }
        retval[i] = &sipAddressHF{ Address : addr }
    }
    return retval, nil
}

func (self *sipAddressHF) getCopy() *sipAddressHF {
    return &sipAddressHF{
        Address : self.Address.GetCopy(),
    }
}

func (self *sipAddressHF) GetUrl() *SipURL {
    return self.Address.url
}
