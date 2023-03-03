package sippy_security

import (
    "testing"
    "time"

    "github.com/sippy/go-b2bua/sippy/time"
)

func TestWrongAlgo(t *testing.T) {
    for name1, alg1 := range algorithms {
        for name2, alg2 := range algorithms {
            if alg1.Mask == alg2.Mask {
                continue
            }
            now, _ := sippy_time.ClockGettime(sippy_time.CLOCK_MONOTONIC)
            nonce := HashOracle.EmitChallenge(alg1.Mask, now)
            if HashOracle.ValidateChallenge(nonce, alg2.Mask, now) {
                t.Errorf("The validation should fail for pair (%s, %s)", name1, name2)
                return
            }
        }
    }
}

func TestExpiration(t *testing.T) {
    for name, alg := range algorithms {
        now, _ := sippy_time.ClockGettime(sippy_time.CLOCK_MONOTONIC)
        nonce := HashOracle.EmitChallenge(alg.Mask, now)
        if ! HashOracle.ValidateChallenge(nonce, alg.Mask, now) {
            t.Errorf("Expiration Test #1 failed for %s", name)
            return
        }
        if ! HashOracle.ValidateChallenge(nonce, alg.Mask, now.Add(30 * time.Second)) {
            t.Errorf("Expiration Test #2 failed for %s", name)
            return
        }
        if HashOracle.ValidateChallenge(nonce, alg.Mask, now.Add(33 * time.Second)) {
            t.Errorf("Expiration Test #3 failed for %s", name)
        }
    }
}

func TestHashOracle(t *testing.T) {
    for name, alg := range algorithms {
        for i := 0; i < 10000; i++ {
            now, _ := sippy_time.ClockGettime(sippy_time.CLOCK_MONOTONIC)
            cryptic := HashOracle.EmitChallenge(alg.Mask, now)
            if ! HashOracle.ValidateChallenge(cryptic, alg.Mask, now) {
                t.Errorf("Algorithm %s failed", name)
            }
        }
    }
}
