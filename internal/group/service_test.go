package group

import (
	"reflect"
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		in       string
		want     string
		loopback bool
		wantErr  bool
	}{
		{in: "192.168.1.50:25565", want: "192.168.1.50:25565"},
		{in: " 10.0.0.5:7777 ", want: "10.0.0.5:7777"},
		{in: "127.0.0.1:25565", want: "127.0.0.1:25565", loopback: true},
		{in: "0.0.0.0:25565", wantErr: true},
		{in: "example.com:25565", wantErr: true},
		{in: "192.168.1.50", wantErr: true},
		{in: "192.168.1.50:0", wantErr: true},
		{in: "192.168.1.50:70000", wantErr: true},
	}
	for _, tt := range tests {
		got, loopback, err := ParseTarget(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseTarget(%q) = %q, want an error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseTarget(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if loopback != tt.loopback {
			t.Errorf("ParseTarget(%q) loopback = %v, want %v", tt.in, loopback, tt.loopback)
		}
	}
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		in      string
		want    []int
		wantErr bool
	}{
		{in: "25565", want: []int{25565}},
		{in: "2456-2458", want: []int{2456, 2457, 2458}},
		{in: "25566,25565", want: []int{25565, 25566}},
		{in: "25565, 25565", want: []int{25565}},
		{in: "", wantErr: true},
		{in: "0", wantErr: true},
		{in: "70000", wantErr: true},
		{in: "2458-2456", wantErr: true},
		{in: "1-9", wantErr: true}, // nine ports, over the cap
		{in: "http", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParsePorts(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParsePorts(%q) = %v, want an error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePorts(%q): %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParsePorts(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		in   []int
		want string
	}{
		{in: []int{25565}, want: "25565"},
		{in: []int{2458, 2456, 2457}, want: "2456-2458"},
		{in: []int{25566, 25565}, want: "25565-25566"},
		{in: []int{25565, 25567}, want: "25565,25567"},
		{in: nil, want: ""},
	}
	for _, tt := range tests {
		if got := FormatPorts(tt.in); got != tt.want {
			t.Errorf("FormatPorts(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
