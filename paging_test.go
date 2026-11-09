package main

import (
	"fmt"
	"testing"
)

// newPagingOrganizer is just enough Organizer for the paging methods, which
// only move altRowoff and redraw.
func newPagingOrganizer(noteLines int) *Organizer {
	sess := &Session{}
	scr := &Screen{screenCols: 100, screenLines: 40, textLines: 30, divider: 40, Session: sess}
	o := &Organizer{Session: sess, Screen: scr}
	for i := range noteLines {
		o.note = append(o.note, fmt.Sprintf("line %d", i))
	}
	return o
}

// Space pages half the pane and stops once the last line is at the bottom;
// b pages back and stops at the top.
func TestPagePreview(t *testing.T) {
	o := newPagingOrganizer(100) // 30-row pane: last full screen starts at 70
	var got []int
	for range 6 {
		o.pagePreviewDown()
		got = append(got, o.altRowoff)
	}
	if want := fmt.Sprint([]int{15, 30, 45, 60, 70, 70}); fmt.Sprint(got) != want {
		t.Errorf("Space offsets = %v, want %v", got, want)
	}
	got = nil
	for range 6 {
		o.pagePreviewUp()
		got = append(got, o.altRowoff)
	}
	if want := fmt.Sprint([]int{55, 40, 25, 10, 0, 0}); fmt.Sprint(got) != want {
		t.Errorf("b offsets = %v, want %v", got, want)
	}

	short := newPagingOrganizer(10) // fits: nothing to page
	short.pagePreviewDown()
	if short.altRowoff != 0 {
		t.Errorf("short note paged to %d", short.altRowoff)
	}
}

func TestPageNotice(t *testing.T) {
	o := newPagingOrganizer(0)
	for i := range 50 {
		o.notice = append(o.notice, fmt.Sprintf("help %d", i))
	}
	// box is 30-10 = 20 rows; a notice that overflows it shows 18
	if got := o.noticeRows(); got != 18 {
		t.Fatalf("noticeRows = %d, want 18", got)
	}
	var got []int
	for range 5 {
		o.pageNoticeDown()
		got = append(got, o.altRowoff)
	}
	if want := fmt.Sprint([]int{9, 18, 27, 32, 32}); fmt.Sprint(got) != want {
		t.Errorf("Space offsets = %v, want %v", got, want)
	}
	o.pageNoticeUp()
	o.pageNoticeUp()
	o.pageNoticeUp()
	o.pageNoticeUp()
	if o.altRowoff != 0 {
		t.Errorf("b left the notice at %d", o.altRowoff)
	}
}

// The organizer dispatches Space, b, PgDn and PgUp to the half-page pager.
func TestPagingKeysAreRegistered(t *testing.T) {
	o := newPagingOrganizer(0)
	cmds := (&App{}).setOrganizerNormalCmds(o)
	for _, key := range []int{' ', 'b', PAGE_DOWN, PAGE_UP, ctrlKey('j'), ctrlKey('k')} {
		if _, ok := cmds[string(rune(key))]; !ok {
			t.Errorf("key %d is not an organizer normal command", key)
		}
	}
}
