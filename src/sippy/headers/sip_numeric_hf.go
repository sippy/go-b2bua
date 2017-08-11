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
    "strconv"
)

type sipNumericHF struct {
    string_body     string
    number          int
    parsed          bool
}

func newSipNumericHF(num int) sipNumericHF {
    return sipNumericHF{
        number      : num,
        parsed      : true,
    }
}

func createSipNumericHF(body string) sipNumericHF {
    return sipNumericHF{
        string_body     : body,
        parsed          : false,
    }
}

func (self *sipNumericHF) StringBody() string {
    if self.parsed {
        return strconv.Itoa(self.number)
    }
    return self.string_body
}

func (self *sipNumericHF) parse() error {
    if ! self.parsed {
        var err error
        self.number, err = strconv.Atoi(self.string_body)
        if err != nil {
            return err
        }
        self.parsed = true
    }
    return nil
}

func (self *sipNumericHF) Number() (int, error) {
    if ! self.parsed {
        if err := self.parse(); err != nil {
            return 0, err
        }
    }
    return self.number, nil
}
