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
//  #include <time.h>
//
//  typedef struct {
//      struct timespec ts;
//      int res;
//  } clock_gettime_result;
//
//  clock_gettime_result 
//  clock_getrealtime() {
//      clock_gettime_result res;
//      res.res = clock_gettime(CLOCK_REALTIME, &res.ts);
//      return res;
//  }
//
//  clock_gettime_result
//  clock_getmonotime() {
//      clock_gettime_result res;
//      res.res = clock_gettime(CLOCK_MONOTONIC, &res.ts);
//      return res;
//  }
//
import "C"
import (
    "errors"
    "fmt"
    "time"
    "sippy/math"
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
    rec_filter sippy_math.RecFilter
}

var globals *monoGlobals

func init() {
    var monot time.Time
    var realt time.Time

    res, _ := C.clock_getmonotime()
    if res.res == 0 {
        monot = time.Unix(int64(res.ts.tv_sec), int64(res.ts.tv_nsec))
    }
    res, _ = C.clock_getrealtime()
    if res.res == 0 {
        realt = time.Unix(int64(res.ts.tv_sec), int64(res.ts.tv_nsec))
    }
    globals = &monoGlobals{
        monot_max : monot,
        rec_filter : sippy_math.NewRecFilter(0.99, realt.Sub(monot).Seconds()),
    }
}

func (self *monoGlobals) Apply(realt, monot time.Time) time.Duration {
    diff_flt := self.rec_filter.Apply(realt.Sub(monot).Seconds())
    if self.monot_max.Before(monot) {
        self.monot_max = monot
    }
    return time.Duration(diff_flt * float64(time.Second))
}

func NewMonoTime2(monot time.Time, realt time.Time) (*MonoTime) {
    return &MonoTime{
        monot : monot,
        realt : realt,
    }
}

func NewMonoTime() (self *MonoTime, err error) {
    res, _ := C.clock_getmonotime()
    if res.res != 0 {
        return nil, errors.New("Cannot read monolitic clock")
    }
    monot := time.Unix(int64(res.ts.tv_sec), int64(res.ts.tv_nsec))

    res, _ = C.clock_getrealtime()
    if res.res != 0 {
        return nil, errors.New("Cannot read realtime clock")
    }
    realt := time.Unix(int64(res.ts.tv_sec), int64(res.ts.tv_nsec))
    diff_flt := globals.Apply(realt, monot)
    realt = monot.Add(diff_flt)

    return &MonoTime{
        monot : monot,
        realt : realt,
    }, nil
}

func (self *MonoTime) Ftime() string {
    t := RoundTime(self.realt).UTC()
    return fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d+00", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

func (self *MonoTime) Fptime() string {
    t := self.realt.UTC()
    return fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d.%06d+00", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond() / 1000)
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
    res, _ := C.clock_getmonotime()
    if res.res != 0 {
        return 0, errors.New("Cannot read monolitic clock")
    }
    monot_now := time.Unix(int64(res.ts.tv_sec), int64(res.ts.tv_nsec))
    return monot_now.Sub(self.monot), nil
}

func (self *MonoTime) GetOffsetCopy(offset time.Duration) *MonoTime {
    return &MonoTime{
        monot : self.monot.Add(offset),
        realt : self.realt.Add(offset),
    }
}
