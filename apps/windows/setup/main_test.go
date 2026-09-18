//go:build windows

package main

import "testing"

func TestNextInstallerFocusWraps(t *testing.T) {
	tests := []struct {
		current int
		delta   int
		want    int
	}{
		{4, 1, 0},
		{0, -1, 4},
		{2, 1, 3},
		{2, -1, 1},
	}
	for _, tt := range tests {
		if got := nextInstallerFocus(tt.current, tt.delta); got != tt.want {
			t.Fatalf("nextInstallerFocus(%d, %d) = %d; want %d", tt.current, tt.delta, got, tt.want)
		}
	}
}

func TestInstallerCheckboxHitTargetsIncludeLabels(t *testing.T) {
	if !inside(400, 234, desktopHitRect) {
		t.Fatal("desktop checkbox label area must be clickable")
	}
	if !inside(400, 272, runHitRect) {
		t.Fatal("run-after checkbox label area must be clickable")
	}
	if !inside(400, 310, startupHitRect) {
		t.Fatal("startup checkbox label area must be clickable")
	}
	if inside(400, 350, startupHitRect) {
		t.Fatal("startup hit target must not extend into install location text")
	}
}
