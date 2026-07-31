package main

import "testing"

func TestIsLoopbackListenAddr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:50051", true},
		{"127.0.0.2:1", true},
		{"[::1]:50051", true},
		{"localhost:50051", true},
		{"LOCALHOST:50051", true},
		{":50051", false},
		{"0.0.0.0:50051", false},
		{"192.168.1.10:50051", false},
		{"[::]:50051", false},
		{"not-a-valid", false},
	}
	for _, tc := range cases {
		if got := isLoopbackListenAddr(tc.addr); got != tc.want {
			t.Fatalf("isLoopbackListenAddr(%q)=%v want %v", tc.addr, got, tc.want)
		}
	}
}
