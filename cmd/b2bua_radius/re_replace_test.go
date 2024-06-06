package main

import (
    "testing"
)

func Test_re_replace(t *testing.T) {
    test_cases := []struct{ inp string; ptrn string; expect string }{
        { "12345671234567", "s/1/_/", "_2345671234567" },
        { "12345671234567", "s/1/_/g", "_234567_234567" },
        { "12345671234567", "s/1/_/g;s/2/#/", "_#34567_234567" },
        { "12345671234567", "s/1/_/;s/2/#/g", "_#345671#34567" },
    }
    for _, tc := range test_cases {
        res, err := re_replace(tc.ptrn, tc.inp)
        if err != nil {
            t.Fatalf("Error applying '%s' to '%s': %s", tc.ptrn, tc.inp, err.Error())
        } else if res != tc.expect {
            t.Fatalf("Expected '%s', got '%s'", tc.expect, res)
        } else {
            t.Logf("ok: '%s' %% '%s' -> '%s'", tc.inp, tc.ptrn, res)
        }
    }
}
