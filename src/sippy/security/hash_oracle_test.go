package sippy_security

import (
    "testing"
    "time"
)

func check_expiration(t *testing.T, nonce string, res_ch chan bool, alg *Algorithm, name string) {
    defer func() { res_ch <- true }()

    if ! HashOracle.ValidateChallenge(nonce, alg.Mask) {
        t.Errorf("Expiration Test #1 Failed for %s ", name)
        return
    }
    time.Sleep(30 * time.Second)
    if ! HashOracle.ValidateChallenge(nonce, alg.Mask) {
        t.Errorf("Expiration Test #2 Failed for %s ", name)
        return
    }
    time.Sleep(2 * time.Second)
    if HashOracle.ValidateChallenge(nonce, alg.Mask) {
        t.Errorf("Expiration Test #3 Failed for %s ", name)
    }
}

func TestExpiration(t *testing.T) {
    res_ch := make(chan bool, 1)

    for name, alg := range algorithms {
        nonce := HashOracle.EmitChallenge(alg.Mask)
        go check_expiration(t, nonce, res_ch, alg, name)
    }
    for range algorithms {
        <-res_ch
    }
}

func TestHashOracle(t *testing.T) {
    for name, alg := range algorithms {
        for i := 0; i < 10000; i++ {
            cryptic := HashOracle.EmitChallenge(alg.Mask)
            if ! HashOracle.ValidateChallenge(cryptic, alg.Mask) {
                t.Errorf("Algorithm %s failed", name)
            }
        }
    }
}
