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

package sippy_log

import (
    "os"
    "strconv"
    "syscall"
    "time"
    "sippy/time"
)

type SipLogger interface {
    Write(rtime *sippy_time.MonoTime, call_id string, msg string)
}

type sipLogger struct {
    fname   string
    id      string
    fd      *os.File
}

func NewSipLogger(id, fname string) (*sipLogger, error) {
    self := &sipLogger{
        fname   : fname,
        id      : id,
    }
    err := self.Reopen()
    if err != nil {
        return nil, err
    }
    return self, nil
}

func _fmt0Xd(v, width int) string {
    ret := strconv.Itoa(v)
    for len(ret) < width {
        ret = "0" + ret
    }
    return ret
}

func FormatDate(t time.Time) string {
    return strconv.Itoa(t.Day()) + " " + t.Month().String()[:3] + " " +
        _fmt0Xd(t.Hour(), 2) + ":" + _fmt0Xd(t.Minute(), 2) + ":" + _fmt0Xd(t.Second(), 2) + "." +
        _fmt0Xd(t.Nanosecond() / 1000000, 3)
}

func (self *sipLogger) Write(rtime *sippy_time.MonoTime, call_id string, msg string) {
    var t time.Time
    if rtime != nil {
        t = rtime.Realt()
    } else {
        t = time.Now()
    }
    //buf := fmt.Sprintf("%d %s %02d:%02d:%06.3f/%s/%s: %s\n",
    buf := FormatDate(t) + "/" + call_id + "/" + self.id + ": " + msg
    fileno := int(self.fd.Fd())
    syscall.Flock(fileno, syscall.LOCK_EX)
    defer syscall.Flock(fileno, syscall.LOCK_UN)
    self.fd.Write([]byte(buf))
}

func (self *sipLogger) Reopen() error {
    var err error
    if self.fd == nil {
        self.fd.Close()
        self.fd = nil
    }
    self.fd, err = os.OpenFile(self.fname, os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
    return err
}
