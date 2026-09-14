package capture

import "testing"

func TestWin10ForcesGDI(t *testing.T) {
	orig := WindowsBuild
	defer func() { WindowsBuild = orig }()

	cases := []struct {
		build    uint32
		wantGDI  bool
		desc     string
	}{
		{19045, true, "Win10 22H2 (19045)"},
		{19041, true, "Win10 21H1 (19041)"},
		{15063, true, "Win10 1703 (15063)"},
		{0, false, "unknown build (fallback)"},
		{22000, false, "Win11 21H2 (22000)"},
		{22631, false, "Win11 23H2 (22631)"},
		{26100, false, "Win11 24H2 (26100)"},
	}

	for _, tc := range cases {
		WindowsBuild = func() uint32 { return tc.build }

		got := AutoBackend()
		gotGDI := got == BackendGDI
		if gotGDI != tc.wantGDI {
			t.Errorf("%s: AutoBackend()=%v, want GDI=%v", tc.desc, got, tc.wantGDI)
		}
	}
}

func TestForceGDI(t *testing.T) {
	orig := WindowsBuild
	defer func() { WindowsBuild = orig }()

	WindowsBuild = func() uint32 { return 19045 }
	if !ForceGDI() {
		t.Error("Win10 (19045): ForceGDI() = false, want true")
	}

	WindowsBuild = func() uint32 { return 22000 }
	if ForceGDI() {
		t.Error("Win11 (22000): ForceGDI() = true, want false")
	}
}

func TestWin10SkipsDLL(t *testing.T) {
	orig := WindowsBuild
	defer func() { WindowsBuild = orig }()

	WindowsBuild = func() uint32 { return 19045 }

	b, warn, err := NewIBackend("auto")
	if err != nil {
		t.Fatalf("auto backend failed: %v", err)
	}
	if b != nil && warn != "" {
		t.Logf("Warning: %s", warn)
	}

	b2, warn2, err2 := NewIBackend("wgc")
	if err2 != nil {
		t.Fatalf("wgc backend failed: %v", err2)
	}
	if warn2 == "" {
		t.Error("Win10 wgc: expected warning about fallback")
	}
	if b2 == nil {
		t.Error("Win10 wgc: expected non-nil backend (GDI fallback)")
	}
}
