package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
)

func TestMoveFocusWraps(t *testing.T) {
	inputs := []textinput.Model{textinput.New(), textinput.New(), textinput.New()}
	idx := 0
	moveFocus(inputs, &idx, -1)
	if idx != 2 || !inputs[2].Focused() || inputs[0].Focused() {
		t.Fatalf("backward wrap: idx=%d", idx)
	}
	moveFocus(inputs, &idx, 1)
	if idx != 0 || !inputs[0].Focused() {
		t.Fatalf("forward wrap: idx=%d", idx)
	}
}

func TestStaleClipboardTimerIgnored(t *testing.T) {
	m := model{clipSeq: 2, message: "keep"}
	next, _ := m.Update(clearClipboardMsg{seq: 1})
	if next.(model).message != "keep" {
		t.Fatal("older timer must not clear a newer copy")
	}
}
