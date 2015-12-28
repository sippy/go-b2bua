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
package sippy_utils

import (
    "unicode"
)

// There is no FieldsN function so here is a substitution for it
func FieldsN(s string, max_slices int) []string {
    return FieldsNFunc(s, max_slices, unicode.IsSpace)
}

func FieldsNFunc(s string, max_slices int, test_func func(rune) bool) []string {
    ret := []string{}
    buf := make([]rune, 0, len(s))
    non_space_found := false
    for _, r := range s {
        if max_slices == 0 {
            buf = append(buf, r)
            continue
        }
        if test_func(r) {
            if non_space_found {
                non_space_found = false
                ret = append(ret, string(buf))
                buf = make([]rune, 0, len(s))
            }
        } else {
            if !non_space_found {
                non_space_found = true
                if max_slices > 0 {
                    max_slices--
                }
            }
            buf = append(buf, r)
        }
    }
    if len(buf) > 0 {
        ret = append(ret, string(buf))
    }
    return ret
}
