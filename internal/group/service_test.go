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
		{"localhost", "127.0.0.1", true},
		{"LOCALHOST", "127.0.0.1", true},
		{"localhost:25565", "127.0.0.1:25565", true},
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

func TestParseTargetIP(t *testing.T) {
	valid := []struct {
		in   string
		want string
	}{
		{"192.168.1.50", "192.168.1.50"},
		{"127.0.0.1", "127.0.0.1"},
		{"localhost", "127.0.0.1"},
		{"LocalHost", "127.0.0.1"},
		{"::1", "::1"},
	}
	for _, tc := range valid {
		got, err := ParseTargetIP(tc.in)
		if err != nil {
			t.Errorf("ParseTargetIP(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseTargetIP(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	invalid := []string{
		"",
		"example.com",
		"0.0.0.0",
		"224.0.0.1",
		"255.255.255.255",
		"192.168.1.50:25565",
		"[::1]:25565",
	}
	for _, in := range invalid {
		if _, err := ParseTargetIP(in); err == nil {
			t.Errorf("ParseTargetIP(%q) accepted an invalid value", in)
		}
	}
}

func TestGameHost(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"192.168.1.50", "192.168.1.50"},
		{"192.168.1.50:25565", "192.168.1.50"},
		{"localhost", "127.0.0.1"},
		{"localhost:25565", "127.0.0.1"},
	}
	for _, tc := range cases {
		got, err := (Service{Target: tc.target}).GameHost()
		if err != nil {
			t.Errorf("GameHost(%q): %v", tc.target, err)
			continue
		}
		if got != tc.want {
			t.Errorf("GameHost(%q) = %q, want %q", tc.target, got, tc.want)
		}
	}
	if _, err := (Service{Target: "example.com"}).GameHost(); err == nil {
		t.Fatal("GameHost accepted a hostname")
	}
}

func TestComposeServiceTarget(t *testing.T) {
	cases := []struct {
		name        string
		ip          string
		gamePorts   []int
		friendPorts []int
		wantTarget  string
		wantPorts   []int
	}{
		{
			name:       "same single port",
			ip:         "192.168.1.50",
			gamePorts:  []int{25565},
			wantTarget: "192.168.1.50",
			wantPorts:  []int{25565},
		},
		{
			name:       "same range",
			ip:         "192.168.1.50",
			gamePorts:  []int{2456, 2457, 2458},
			wantTarget: "192.168.1.50",
			wantPorts:  []int{2456, 2457, 2458},
		},
		{
			name:       "same list",
			ip:         "192.168.1.50",
			gamePorts:  []int{2456, 2600},
			wantTarget: "192.168.1.50",
			wantPorts:  []int{2456, 2600},
		},
		{
			name:        "remapped single port",
			ip:          "192.168.1.50",
			gamePorts:   []int{25566},
			friendPorts: []int{25565},
			wantTarget:  "192.168.1.50:25566",
			wantPorts:   []int{25565},
		},
		{
			name:        "remapped loopback",
			ip:          "127.0.0.1",
			gamePorts:   []int{25566},
			friendPorts: []int{25565},
			wantTarget:  "127.0.0.1:25566",
			wantPorts:   []int{25565},
		},
		{
			name:       "localhost alias",
			ip:         "localhost",
			gamePorts:  []int{25565},
			wantTarget: "127.0.0.1",
			wantPorts:  []int{25565},
		},
	}
	for _, tc := range cases {
		target, ports, err := ComposeServiceTarget(tc.ip, tc.gamePorts, tc.friendPorts)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if target != tc.wantTarget || !reflect.DeepEqual(ports, tc.wantPorts) {
			t.Errorf("%s = %q, %v; want %q, %v", tc.name, target, ports, tc.wantTarget, tc.wantPorts)
		}
	}

	invalid := []struct {
		name        string
		ip          string
		gamePorts   []int
		friendPorts []int
	}{
		{"no game ports", "192.168.1.50", nil, nil},
		{"bad ip", "example.com", []int{25565}, nil},
		{"ip with port", "192.168.1.50:25565", []int{25565}, nil},
		{"one friend port for a range", "192.168.1.50", []int{2456, 2457}, []int{25565}},
		{"many friend ports", "192.168.1.50", []int{25566}, []int{25565, 25566}},
		{"duplicate game ports", "192.168.1.50", []int{25565, 25565}, nil},
		{"invalid game port", "192.168.1.50", []int{0}, nil},
	}
	for _, tc := range invalid {
		if _, _, err := ComposeServiceTarget(tc.ip, tc.gamePorts, tc.friendPorts); err == nil {
			t.Errorf("%s was accepted", tc.name)
		}
	}
}

func TestServiceGamePorts(t *testing.T) {
	same := Service{Name: "Valheim", Proto: "udp", Target: "192.168.1.50", Ports: []int{2456, 2457}}
	got, err := same.GamePorts()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{2456, 2457}) {
		t.Fatalf("same game ports = %v", got)
	}

	remap := Service{Name: "Remap", Proto: "tcp", Target: "192.168.1.50:25566", Ports: []int{25565}}
	got, err = remap.GamePorts()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{25566}) {
		t.Fatalf("remap game ports = %v", got)
	}

	bad := Service{Name: "Bad", Proto: "tcp", Target: "example.com", Ports: []int{25565}}
	if _, err := bad.GamePorts(); err == nil {
		t.Fatal("an invalid target was accepted")
	}
}

func TestServiceGameAddress(t *testing.T) {
	cases := []struct {
		service Service
		want    string
	}{
		{Service{Target: "192.168.1.50", Ports: []int{2456, 2457}}, "192.168.1.50:2456-2457"},
		{Service{Target: "192.168.1.50:25566", Ports: []int{25565}}, "192.168.1.50:25566"},
		{Service{Target: "::1", Ports: []int{25565}}, "[::1]:25565"},
	}
	for _, tc := range cases {
		got, err := tc.service.GameAddress()
		if err != nil {
			t.Errorf("%q: %v", tc.service.Target, err)
			continue
		}
		if got != tc.want {
			t.Errorf("GameAddress(%q, %v) = %q, want %q", tc.service.Target, tc.service.Ports, got, tc.want)
		}
	}

	bad := Service{Target: "example.com", Ports: []int{25565}}
	if _, err := bad.GameAddress(); err == nil {
		t.Fatal("an invalid target was accepted")
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
