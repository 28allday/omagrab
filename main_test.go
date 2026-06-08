package main

import "testing"

func TestNormalizeArgURL(t *testing.T) {
	want := "https://example.com/watch?v=abc123&t=10s"
	cases := map[string]string{
		"plain https":     want,
		"omagrab scheme":  "omagrab:https%3A%2F%2Fexample.com%2Fwatch%3Fv%3Dabc123%26t%3D10s",
		"omagrab slashes": "omagrab://https%3A%2F%2Fexample.com%2Fwatch%3Fv%3Dabc123%26t%3D10s",
		"unescaped scheme": "omagrab:https://example.com/watch?v=abc123&t=10s",
	}
	for name, in := range cases {
		if got := normalizeArgURL(in); got != want {
			t.Errorf("%s: normalizeArgURL(%q) = %q, want %q", name, in, got, want)
		}
	}
	for name, in := range map[string]string{
		"junk":      "not a url",
		"ftp":       "ftp://example.com",
		"empty":     "",
		"scheme only": "omagrab:",
	} {
		if got := normalizeArgURL(in); got != "" {
			t.Errorf("%s: normalizeArgURL(%q) = %q, want empty", name, in, got)
		}
	}
}
