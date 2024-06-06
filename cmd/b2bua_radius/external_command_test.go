package main

import (
    "testing"
    "time"

    "github.com/sippy/go-b2bua/sippy/log"
)

func Test_ExternalCommand(t *testing.T) {
    logger := sippy_log.NewErrorLogger()
    ec := newExternalCommand(1, logger, "/bin/cat")
    res_ch := make(chan []string, 1)
    result_cb := func(res []string) {
        res_ch <- res
    }
    for _, s := range []string{ "foo", "bar" } {
        var res []string

        ec.process_command([]string{ s }, result_cb)
        select {
        case res = <-res_ch:
        case <-time.After(500 * time.Millisecond):
            t.Fatal("Timeout waiting for response")
            return
        }
        if len(res) == 0 {
            t.Fatalf("Empty response for input '%s'", s)
        } else if s != res[0] {
            t.Fatalf("Expected '%s', received '%s'", s, res[0])
        } else {
            t.Logf("ok: sent '%s', got '%s'", s, res[0])
        }
    }
}
