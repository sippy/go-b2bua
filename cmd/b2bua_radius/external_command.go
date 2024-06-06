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
    "bufio"
    "io"
    "os/exec"
    "strings"

    "github.com/sippy/go-b2bua/sippy/log"
    "github.com/sippy/go-b2bua/sippy/utils"
)

type ExternalCommand struct {
    work_ch         chan *Work_item
}

type Work_item struct {
    data            []string
    result_callback func([]string)
    cancelled       bool
}

type Worker struct {
    master          *ExternalCommand
    command         string
    args            []string
    logger          sippy_log.ErrorLogger
}

func newWorker(master *ExternalCommand, logger sippy_log.ErrorLogger, command string, args []string) *Worker {
    return &Worker{
        master      : master,
        command     : command,
        args        : args,
        logger      : logger,
    }
}

func (self *Worker) run() {
    var stdout_raw io.ReadCloser
    var stdin io.WriteCloser
    var err error

    cmd := exec.Command(self.command, self.args...)
    if stdin, err = cmd.StdinPipe(); err != nil {
        self.logger.Error("ExternalCommand Worker: cannot get stdin pipe: " + err.Error())
        return
    }
    if stdout_raw, err = cmd.StdoutPipe(); err != nil {
        self.logger.Error("ExternalCommand Worker: cannot get stdout pipe: " + err.Error())
        return
    }
    err = cmd.Start()
    if err != nil {
        self.logger.Error("ExternalCommand Worker: " + err.Error())
        return
    }
    defer cmd.Wait()
    stdout := bufio.NewReader(stdout_raw)
    for {
        wi := <-self.master.work_ch
        if wi == nil {
            break
        }
        if wi.cancelled {
            wi.data = nil
            wi.result_callback = nil
            continue
        }
        batch := []byte(strings.Join(wi.data, "\n") + "\n\n")
        stdin.Write(batch)
        result := []string{}
        for {
            var buf, line []byte
            var is_prefix bool

            buf, is_prefix, err = stdout.ReadLine()
            if err != nil {
                break
            }
            line = append(line, buf...)
            if is_prefix {
                continue
            }
            s := strings.TrimSpace(string(line))
            if len(s) == 0 {
                break
            }
            result = append(result, s)
            line = []byte{}
        }
        result_callback := wi.result_callback
        if result_callback != nil {
            sippy_utils.SafeCall(func() { wi.result_callback(result) }, nil, self.logger)
        }
        wi.data = nil
        wi.result_callback = nil
    }
}

func newWork_item(data []string, result_callback func([]string)) *Work_item {
    return &Work_item{
        data            : data,
        result_callback : result_callback,
        cancelled       : false,
    }
}

func (self *Work_item) Cancel() {
    self.cancelled = true
}

func newExternalCommand(max_workers int, logger sippy_log.ErrorLogger, cmd string, opts ...string) *ExternalCommand {
    self := &ExternalCommand{
        work_ch             : make(chan *Work_item, 1000),
    }
    for i := 0; i < max_workers; i++ {
        w := newWorker(self, logger, cmd, opts)
        go w.run()
    }
    return self
}
func (self *ExternalCommand) process_command(data []string, result_callback func([]string)) Cancellable {
    wi := newWork_item(data, result_callback)
    self.work_ch <- wi
    return wi
}
