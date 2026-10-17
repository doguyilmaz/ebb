package units

import "testing"

func TestBytes(t *testing.T) {
	for in, want := range map[uint64]string{0: "0 B", 999: "999 B", 1500: "1.5 kB", 35_200_000_000: "35.2 GB", 2_000_000: "2 MB"} {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintable(t *testing.T) {
	if got := Printable("a\x1b[31mb\nc\u009bd"); got != "a?[31mb?c?d" {
		t.Fatalf("%q", got)
	}
	if got := Printable("plain/é"); got != "plain/é" {
		t.Fatal(got)
	}
}
