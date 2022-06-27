// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
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

package sippy_time
//
// #cgo linux LDFLAGS: -lrt
//
// #include <time.h>
//
// typedef struct timespec timespec_struct;
//
import "C"
import (
    "errors"
    "strings"
    "strconv"
    "time"

    "sippy/math"
    "sippy/fmt"
)

const (
    CLOCK_REALTIME = C.CLOCK_REALTIME
    CLOCK_MONOTONIC = C.CLOCK_MONOTONIC
)

type MonoTime struct {
    monot time.Time
    realt time.Time
}

type GoroutineCtx interface {
    Apply(realt, monot time.Time) time.Duration
}

type monoGlobals struct {
    monot_max time.Time
    realt_flt sippy_math.RecFilter
}

var globals *monoGlobals

func init() {
    t, _ := newMonoTime()
    globals = &monoGlobals{
        monot_max : t.monot,
        realt_flt : sippy_math.NewRecFilter(0.99, t.realt.Sub(t.monot).Seconds()),
    }
}

func (self *monoGlobals) Apply(realt, monot time.Time) time.Duration {
    diff_flt := self.realt_flt.Apply(realt.Sub(monot).Seconds())
    if self.monot_max.Before(monot) {
        self.monot_max = monot
    }
    return time.Duration(diff_flt * float64(time.Second))
}

func NewMonoTimeFromString(s string) (*MonoTime, error) {
    parts := strings.SplitN(s, "-", 2)
    realt0, err := strconv.ParseFloat(parts[0], 64)
    if err != nil {
        return nil, err
    }
    realt := FloatToTime(realt0)
    if len(parts) == 1 {
        return NewMonoTime1(realt)
    }
    monot0, err := strconv.ParseFloat(parts[1], 64)
    if err != nil {
        return nil, err
    }
    monot := FloatToTime(monot0)
    return NewMonoTime2(monot, realt), nil
}

func NewMonoTime1(realt time.Time) (*MonoTime, error) {
    monot := TimeToFloat(realt) - globals.realt_flt.GetLastval()
    self := &MonoTime{
        realt : realt,
        monot : FloatToTime(monot),
    }
    if self.monot.After(globals.monot_max) {
        var ts C.timespec_struct
        if res, _ := C.clock_gettime(C.CLOCK_REALTIME, &ts); res != 0 {
            return nil, errors.New("Cannot read realtime clock")
        }
        monot_now := time.Unix(int64(ts.tv_sec), int64(ts.tv_nsec))
        if monot_now.After(globals.monot_max) {
            globals.monot_max = monot_now
        }
        self.monot = globals.monot_max
    }
    return self, nil
}

func NewMonoTime2(monot time.Time, realt time.Time) (*MonoTime) {
    return &MonoTime{
        monot : monot,
        realt : realt,
    }
}

func newMonoTime() (self *MonoTime, err error) {
    var ts C.timespec_struct

    if res, _ := C.clock_gettime(C.CLOCK_MONOTONIC, &ts); res != 0 {
        return nil, errors.New("Cannot read monolitic clock")
    }
    monot := time.Unix(int64(ts.tv_sec), int64(ts.tv_nsec))

    if res, _ := C.clock_gettime(C.CLOCK_REALTIME, &ts); res != 0 {
        return nil, errors.New("Cannot read realtime clock")
    }
    realt := time.Unix(int64(ts.tv_sec), int64(ts.tv_nsec))
    return &MonoTime{
        monot : monot,
        realt : realt,
    }, nil
}

func ClockGettime(cid C.clockid_t) (time.Time, error) {
    var ts C.timespec_struct

    if res, _ := C.clock_gettime(cid, &ts); res != 0 {
        return time.Unix(0, 0), errors.New("Cannot read clock")
    }
    return time.Unix(int64(ts.tv_sec), int64(ts.tv_nsec)), nil
}

func NewMonoTime() (self *MonoTime, err error) {
    t, err := newMonoTime()
    if err != nil {
        return nil, err
    }
    diff_flt := globals.Apply(t.realt, t.monot)
    t.realt = t.monot.Add(diff_flt)
    return t, nil
}

func (self *MonoTime) Ftime() string {
    t := RoundTime(self.realt).UTC()
    return sippy_fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d+00", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

func (self *MonoTime) Fptime() string {
    t := self.realt.UTC()
    return sippy_fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d.%06d+00", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond() / 1000)
}

func (self *MonoTime) Monot() time.Time {
    return self.monot
}

func (self *MonoTime) Realt() time.Time {
    return self.realt
}

func (self *MonoTime) Add(d time.Duration) (*MonoTime) {
    return NewMonoTime2(self.monot.Add(d), self.realt.Add(d))
}

func (self *MonoTime) Sub(t *MonoTime) (time.Duration) {
    return self.monot.Sub(t.monot)
}

func (self *MonoTime) After(t *MonoTime) bool {
    return self.monot.After(t.monot)
}

func (self *MonoTime) Before(t *MonoTime) bool {
    return self.monot.Before(t.monot)
}

func (self *MonoTime) OffsetFromNow() (time.Duration, error) {
    var ts C.timespec_struct

    if res, _ := C.clock_gettime(C.CLOCK_MONOTONIC, &ts); res != 0 {
        return 0, errors.New("Cannot read monolitic clock")
    }
    monot_now := time.Unix(int64(ts.tv_sec), int64(ts.tv_nsec))
    return monot_now.Sub(self.monot), nil
}

func (self *MonoTime) GetOffsetCopy(offset time.Duration) *MonoTime {
    return &MonoTime{
        monot : self.monot.Add(offset),
        realt : self.realt.Add(offset),
    }
}
