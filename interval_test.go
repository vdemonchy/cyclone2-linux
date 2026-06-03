package main

import (
	"testing"
	"time"
)

func TestResolveInterval(t *testing.T) {
	cases := []struct {
		flag, env string
		cfg       int
		want      time.Duration
		wantErr   bool
	}{
		{"", "", 0, 60 * time.Second, false},        // default
		{"", "", 30, 30 * time.Second, false},        // config layer
		{"", "45s", 30, 45 * time.Second, false},     // env beats config
		{"10s", "45s", 30, 10 * time.Second, false},  // flag beats all
		{"", "", 2, 0, true},                         // config below 5s floor
		{"2s", "", 0, 0, true},                       // flag below floor
		{"garbage", "", 0, 0, true},                  // unparseable flag
	}
	for _, c := range cases {
		got, err := resolveInterval(c.flag, c.env, c.cfg)
		if (err != nil) != c.wantErr {
			t.Fatalf("flag=%q env=%q cfg=%d err=%v wantErr=%v", c.flag, c.env, c.cfg, err, c.wantErr)
		}
		if err == nil && got != c.want {
			t.Fatalf("flag=%q env=%q cfg=%d got %v want %v", c.flag, c.env, c.cfg, got, c.want)
		}
	}
}
