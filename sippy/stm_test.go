package sippy

import (
    "testing"
    "runtime"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/log"
)

func Test_SipTransactionManager(t *testing.T) {
    var err error

    error_logger := sippy_log.NewErrorLogger()
    sip_logger := NewTestSipLogger()
    config := sippy_conf.NewConfig(error_logger, sip_logger)
    config.SetSipAddress(config.GetMyAddress())
    config.SetSipPort(config.GetMyPort())
    cmap := NewTestCallMap(config)
    tfactory := NewTestSipTransportFactory()
    config.SetSipTransportFactory(tfactory)
    numGoroutinesBefore := runtime.NumGoroutine()
    cmap.sip_tm, err = NewSipTransactionManager(config, cmap)
    if err != nil {
        t.Fatal("Cannot create SIP transaction manager: " + err.Error())
    }
    go cmap.sip_tm.Run()
    cmap.sip_tm.Shutdown()
    numGoroutinesAfter := runtime.NumGoroutine()
    if numGoroutinesBefore != numGoroutinesAfter {
        t.Fatalf("numGoroutinesBefore = %d, numGoroutinesAfter = %d\n", numGoroutinesBefore, numGoroutinesAfter)
    }
}