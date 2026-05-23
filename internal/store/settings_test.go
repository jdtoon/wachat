package store

import (
	"context"
	"testing"
)

func TestSettings_GetReturnsEmptyForMissingKey(t *testing.T) {
	s := openTempStore(t)
	v, err := s.GetSetting(context.Background(), "nope")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if v != "" {
		t.Errorf("GetSetting(missing) = %q, want empty", v)
	}
}

func TestSettings_SetAndGetRoundTrip(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	if err := s.SetSetting(ctx, "theme", "dark"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	v, err := s.GetSetting(ctx, "theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if v != "dark" {
		t.Errorf("GetSetting = %q, want dark", v)
	}
}

func TestSettings_SetOverwrites(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	if err := s.SetSetting(ctx, "density", "comfortable"); err != nil {
		t.Fatalf("first SetSetting: %v", err)
	}
	if err := s.SetSetting(ctx, "density", "compact"); err != nil {
		t.Fatalf("second SetSetting: %v", err)
	}
	v, _ := s.GetSetting(ctx, "density")
	if v != "compact" {
		t.Errorf("overwrite: got %q, want compact", v)
	}
}

func TestSettings_RequiresKey(t *testing.T) {
	s := openTempStore(t)
	if _, err := s.GetSetting(context.Background(), ""); err == nil {
		t.Error("empty key Get: want error")
	}
	if err := s.SetSetting(context.Background(), "", "v"); err == nil {
		t.Error("empty key Set: want error")
	}
}
