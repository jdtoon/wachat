package ui

import (
	"testing"
)

func TestComposer_SetTextAndText(t *testing.T) {
	c := NewComposer()
	c.SetText("draft body")
	if got := c.Text(); got != "draft body" {
		t.Errorf("Text() = %q, want %q", got, "draft body")
	}
}

func TestComposer_ClearEmpties(t *testing.T) {
	c := NewComposer()
	c.SetText("not empty")
	c.Clear()
	if got := c.Text(); got != "" {
		t.Errorf("Text() after Clear = %q, want empty", got)
	}
}

func TestComposer_CanSend(t *testing.T) {
	c := NewComposer()
	if c.canSend() {
		t.Error("canSend() = true on empty editor, want false")
	}
	c.SetText("   \n\t ")
	if c.canSend() {
		t.Error("canSend() = true on whitespace-only editor, want false")
	}
	c.SetText("hello")
	if !c.canSend() {
		t.Error("canSend() = false on non-empty editor, want true")
	}
}

// TestComposer_SubmitTrimsAndClears exercises the shared submit path
// (used by both the button-click and Enter-key code paths).
func TestComposer_SubmitTrimsAndClears(t *testing.T) {
	c := NewComposer()
	c.SetText("  hello world  ")

	var got string
	c.submit(func(body string) { got = body })

	if got != "hello world" {
		t.Errorf("onSend received %q, want %q (trimmed)", got, "hello world")
	}
	if c.Text() != "" {
		t.Errorf("editor not cleared after submit; Text()=%q", c.Text())
	}
}

func TestComposer_SubmitEmptyIsNoOp(t *testing.T) {
	c := NewComposer()
	c.SetText("    ")

	fired := false
	c.submit(func(body string) { fired = true })
	if fired {
		t.Error("submit fired callback on whitespace-only body")
	}
}
