package profiling

import "testing"

func TestStart(t *testing.T) {
	if s, err := Start(""); s != nil || err != nil {
		t.Fatal("disabled profiling must not start")
	}
	for _, addr := range []string{":6060", "0.0.0.0:6060", "[::]:6060", "example.com:6060"} {
		if s, err := Start(addr); err == nil {
			if s != nil {
				s.Close()
			}
			t.Fatalf("accepted public address %s", addr)
		}
	}
	s, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
}
