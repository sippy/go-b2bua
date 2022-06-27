// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
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
package sippy_utils

import (
    "os"
    "os/exec"
    "os/signal"
    "syscall"

    "sippy/log"
)

const (
    ENV_VAR         = "_GO_DAEMON"
    ENV_VAR_VALUE   = "1"
)

var reopen_funcs []func() = []func(){}

func reopen_logfile(fname string, uid, gid int, logger sippy_log.ErrorLogger) error {
    logfd, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
    if err != nil {
        return err
    }
    syscall.Dup2(int(logfd.Fd()), int(os.Stderr.Fd()))
    syscall.Dup2(int(logfd.Fd()), int(os.Stdout.Fd()))
    logfd.Close()
    os.Chown(fname, uid, gid)
    return nil
}

func AddLogReopenFunc(fn func()) {
    reopen_funcs = append(reopen_funcs, fn)
}

func Daemonize(logfile string, loguid, loggid int, logger sippy_log.ErrorLogger) error {
    if os.Getenv(ENV_VAR) == ENV_VAR_VALUE {
        // Setup log rotation
        AddLogReopenFunc(func() {
            logger.Debug("Signal received, reopening the log file")
            err := reopen_logfile(logfile, loguid, loggid, logger)
            if err != nil {
                logger.Error("Cannot reopen " + logfile + ": " + err.Error())
            }
        })
        sig_ch := make(chan os.Signal, 1)
        signal.Notify(sig_ch, syscall.SIGUSR1)
        go func() {
            for {
                <-sig_ch
                for _, fn := range reopen_funcs {
                    fn()
                }
            }
        }()
        return nil // I am a child
    }
    err := reopen_logfile(logfile, loguid, loggid, logger)
    if err != nil {
        return err
    }
    cmd := exec.Command(os.Args[0], os.Args[1:]...)
    cmd.Env = append(os.Environ(), ENV_VAR + "=" + ENV_VAR_VALUE)
    cmd.Stderr = os.Stderr
    cmd.Stdout = os.Stdout
    cmd.SysProcAttr = &syscall.SysProcAttr{ Setsid : true }
    err = cmd.Start()
    if err != nil {
        logger.Error("Cannot start: " + err.Error())
        os.Exit(1)
    }
    os.Exit(0)
    return nil // not reached
}
