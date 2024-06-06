//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2024 Sippy Software, Inc. All rights reserved.
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
package main

import (
    "strings"
    "regexp"
)

type re_sub_once struct {
    done    bool
    repl    string
}

func new_re_sub_once(repl string) *re_sub_once {
    return &re_sub_once{
        done    : false,
        repl    : repl,
    }
}

func (self *re_sub_once) repl_fn(s string) string {
    if self.done {
        return s
    }
    self.done = true
    return self.repl
}

func re_replace(ptrn, s string) (string, error) {
    s = strings.SplitN(s, "#", 2)[0]
    ptrn_arr := strings.Split(ptrn, "/")
    for len(ptrn_arr) >= 4 {
        p := ptrn_arr[1]
        r := ptrn_arr[2]
        mod := ptrn_arr[3]
        mod = strings.TrimSpace(mod)
        if len(mod) > 0 && mod[0] != ';' {
            ptrn_arr[3] = mod[1:]
            mod = strings.ToLower(mod[:1])
        } else {
            ptrn_arr[3] = mod
        }
        global_replace := false
        for _, c := range mod {
            if c == 'g' {
                global_replace = true
                break
            }
        }
        re, err := regexp.CompilePOSIX(p)
        if err != nil {
            return s, err
        }
        if global_replace {
            s = re.ReplaceAllString(s, r)
        } else {
            re_sub := new_re_sub_once(r)
            s = re.ReplaceAllStringFunc(s, re_sub.repl_fn)
        }
        if len(ptrn_arr) == 4 && ptrn_arr[3] == "" {
            break
        }
        ptrn_arr = ptrn_arr[3:]
    }
    return s, nil
}

