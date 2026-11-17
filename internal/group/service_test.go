package group

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"25565", []int{25565}},
		{"2456-2458,25565", []int{2456, 2457, 2458, 25565}},
		{"25565,25565", []int{25565}},
		{"1-8", []int{1, 2, 3, 4, 5, 6, 7, 8}},
		{" 25565 , 25566 ", []int{25565, 25566}},
	}
	for _, tc := range cases {
		got, err := ParsePorts(tc.in)
		if err != nil {
			t.Errorf("ParsePorts(%q): %v", tc.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParsePorts(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	for _, in := range []string{"", "0", "65536", "3-1", "1-9", "abc"} {
		if _, err := ParsePorts(in); err == nil {
			t.Errorf("ParsePorts(%q) accepted an invalid value", in)
		}
	}
}

func TestFormatPorts(t *testing.T) {
	cases := []struct {
		in   []int
		want string
	}{
		{nil, ""},
		{[]int{25565}, "25565"},
		{[]int{2458, 2456, 2457, 25565}, "2456-2458,25565"},
		{[]int{1, 3, 4}, "1,3-4"},
	}
	for _, tc := range cases {
		if got := FormatPorts(tc.in); got != tc.want {
			t.Errorf("FormatPorts(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseTarget(t *testing.T) {
	valid := []struct {
		in       string
		want     string
		loopback bool
	}{
		{"192.168.1.50", "192.168.1.50", false},
		{"192.168.1.50:25565", "192.168.1.50:25565", false},
		{"127.0.0.1:25565", "127.0.0.1:25565", true},
		{"::1", "::1", true},
		{"[::1]:25565", "[::1]:25565", true},
	}
	for _, tc := range valid {
		got, loopback, err := ParseTarget(tc.in)
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want || loopback != tc.loopback {
			t.Errorf("ParseTarget(%q) = %q, %v", tc.in, got, loopback)
		}
	}

	invalid := []string{
		"",
		"example.com:25565",
		"0.0.0.0:25565",
		"224.0.0.1:25565",
		"255.255.255.255:25565",
		"192.168.1.50:0",
		"192.168.1.50:70000",
		"192.168.1.50:abc",
	}
	for _, in := range invalid {
		if _, _, err := ParseTarget(in); err == nil {
			t.Errorf("ParseTarget(%q) accepted an invalid target", in)
		}
	}
}

func TestValidateService(t *testing.T) {
	valid := []Service{
		{Name: "Minecraft", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25565}},
		{Name: "Valheim", Proto: "udp", Target: "192.168.1.50", Ports: []int{2456, 2457, 2458}},
		{Name: "Remap", Proto: "tcp", Target: "192.168.1.50:25566", Ports: []int{25565}},
	}
	for _, s := range valid {
		if err := ValidateService(s); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}

	invalid := []Service{
		{Name: "", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25565}},
		{Name: "Bad proto", Proto: "http", Target: "192.168.1.50", Ports: []int{25565}},
		{Name: "No ports", Proto: "tcp", Target: "192.168.1.50"},
		{Name: "Duplicate", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25565, 25565}},
		{Name: "Too many", Proto: "tcp", Target: "192.168.1.50", Ports: []int{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{Name: "Ambiguous", Proto: "tcp", Target: "192.168.1.50:25566", Ports: []int{25565, 25566}},
		{Name: "Bad target", Proto: "tcp", Target: "0.0.0.0", Ports: []int{25565}},
	}
	for _, s := range invalid {
		if err := ValidateService(s); err == nil {
			t.Errorf("%s was accepted", s.Name)
		}
	}
}

func TestTargetFor(t *testing.T) {
	rangeService := Service{Name: "Valheim", Proto: "udp", Target: "192.168.1.50", Ports: []int{2456, 2457}}
	got, err := rangeService.TargetFor(2457)
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.1.50:2457" {
		t.Fatalf("range target = %q", got)
	}

	remap := Service{Name: "Remap", Proto: "tcp", Target: "192.168.1.50:25566", Ports: []int{25565}}
	got, err = remap.TargetFor(25565)
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.1.50:25566" {
		t.Fatalf("remap target = %q", got)
	}

	ambiguous := Service{Name: "Bad", Proto: "tcp", Target: "192.168.1.50:25566", Ports: []int{25565, 25566}}
	if _, err := ambiguous.TargetFor(25565); err == nil {
		t.Fatal("ambiguous target was accepted")
	}
}

func TestServiceWarnings(t *testing.T) {
	warnings := ServiceWarnings(Service{
		Name:   "SSH",
		Proto:  "tcp",
		Target: "127.0.0.1:22",
		Ports:  []int{22},
	})
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v", warnings)
	}
}
