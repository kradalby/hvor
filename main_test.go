package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
)

// ============================================================
// Test helpers
// ============================================================

// makePageEvents creates n dummy pageEvents with sequential weekly dates.
func makePageEvents(n int) pageEvents {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	evs := make(pageEvents, n)

	for i := range n {
		evs[i] = pageEvent{
			From:        base.AddDate(0, 0, i*7),
			To:          base.AddDate(0, 0, i*7+1),
			Summary:     fmt.Sprintf("Event %d", i),
			Description: []string{},
		}
	}

	return evs
}

// addAllDayEvent adds an all-day event with the given uid, dates, and summary.
func addAllDayEvent(
	cal *ics.Calendar,
	uid string,
	start, end time.Time,
	summary string,
) {
	event := cal.AddEvent(uid)
	event.SetAllDayStartAt(start)
	event.SetAllDayEndAt(end)
	event.SetSummary(summary)
}

// validICS is a minimal parseable calendar for httptest servers.
const validICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:test-1\r\nDTSTART;VALUE=DATE:20250101\r\n" +
	"DTEND;VALUE=DATE:20250102\r\nSUMMARY:Test Event\r\n" +
	"END:VEVENT\r\nEND:VCALENDAR\r\n"

// trackingBody wraps a ReadCloser and records whether Close was called.
type trackingBody struct {
	io.ReadCloser
	closed bool
}

func (tb *trackingBody) Close() error {
	tb.closed = true

	return tb.ReadCloser.Close()
}

// trackingTransport wraps an http.RoundTripper and tracks response bodies.
type trackingTransport struct {
	base   http.RoundTripper
	bodies []*trackingBody
}

func (tt *trackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := tt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	tb := &trackingBody{ReadCloser: resp.Body}
	tt.bodies = append(tt.bodies, tb)
	resp.Body = tb

	return resp, nil
}

// ============================================================
// Existing test
// ============================================================

func TestSanitiseLocationTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{
			title: "Leiden\\nNetherlands",
			want:  "Leiden, Netherlands",
		},
		{
			title: "London\\nEngland",
			want:  "London, England",
		},
		{
			title: "Memphis, TNnUnited States",
			want:  "Memphis, Tennessee, United States",
		},
		{
			title: "SandefjordnSandefjord Municipality, Norway",
			want:  "Sandefjord, Sandefjord Municipality, Norway",
		},
		{
			title: "Flagstaff, AZnUnited States",
			want:  "Flagstaff, Arizona, United States",
		},
		{
			title: "Berlin\\nGermany",
			want:  "Berlin, Germany",
		},
		{
			title: "St. Louis, MOnUnited States",
			want:  "St. Louis, Missouri, United States",
		},
		{
			title: "Norway",
			want:  "Norway",
		},
	}

	for _, tt := range tests {
		got := sanitiseLocationTitle(tt.title)

		if got != tt.want {
			t.Errorf("unexpected sanitised title, got: %s, want: %s", got, tt.want)
		}
	}
}

// ============================================================
// Group 1: Safety net — locks in current correct behavior
// ============================================================

func TestSanatiseCalText(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello\\, world", "hello, world"},
		{"no escapes", "no escapes"},
		{"a\\, b\\, c", "a, b, c"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := sanitiseCalText(tt.input); got != tt.want {
			t.Errorf("sanitiseCalText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitiseDescription(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"line1\\nline2\\nline3", []string{"line1", "line2", "line3"}},
		{"single line", []string{"single line"}},
		{"", []string{""}},
	}

	for _, tt := range tests {
		got := sanitiseDescription(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("sanitiseDescription(%q): got %d parts, want %d", tt.input, len(got), len(tt.want))

			continue
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("sanitiseDescription(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseTokensValid(t *testing.T) {
	toks := parseTokens("abc,def,ghi")

	if !toks.isValid("abc") {
		t.Error("expected 'abc' to be valid")
	}

	if !toks.isValid("def") {
		t.Error("expected 'def' to be valid")
	}

	if !toks.isValid("ghi") {
		t.Error("expected 'ghi' to be valid")
	}

	if toks.isValid("xyz") {
		t.Error("expected 'xyz' to be invalid")
	}

	if toks.isValid("") {
		t.Error("expected empty string to be invalid with populated token list")
	}
}

func TestGetAppleLocation(t *testing.T) {
	event := ics.NewEvent("test-loc")
	event.AddProperty(
		ics.ComponentProperty("X-APPLE-STRUCTURED-LOCATION"),
		"geo:52.1601,4.4970",
		&ics.KeyValues{Key: "X-TITLE", Value: []string{"Leiden\\nNetherlands"}},
		&ics.KeyValues{Key: "X-APPLE-RADIUS", Value: []string{"5000"}},
	)

	loc := getAppleLocation(event)
	if loc == nil {
		t.Fatal("expected non-nil location")
	}

	if loc.Title != "Leiden, Netherlands" {
		t.Errorf("Title = %q, want %q", loc.Title, "Leiden, Netherlands")
	}

	if loc.Latitude != "52.1601" {
		t.Errorf("Latitude = %q, want %q", loc.Latitude, "52.1601")
	}

	if loc.Longitude != "4.4970" {
		t.Errorf("Longitude = %q, want %q", loc.Longitude, "4.4970")
	}

	if loc.Radius != 5000 {
		t.Errorf("Radius = %f, want %f", loc.Radius, 5000.0)
	}
}

func TestGetAppleLocationNil(t *testing.T) {
	event := ics.NewEvent("test-no-loc")
	event.SetSummary("No Location Event")

	loc := getAppleLocation(event)
	if loc != nil {
		t.Errorf("expected nil location, got %+v", loc)
	}
}

func TestGetAppleLocationNoGeo(t *testing.T) {
	event := ics.NewEvent("test-no-geo")
	event.AddProperty(
		ics.ComponentProperty("X-APPLE-STRUCTURED-LOCATION"),
		"some-non-geo-value",
		&ics.KeyValues{Key: "X-TITLE", Value: []string{"Somewhere"}},
	)

	loc := getAppleLocation(event)
	if loc == nil {
		t.Fatal("expected non-nil location")
	}

	if loc.Title != "Somewhere" {
		t.Errorf("Title = %q, want %q", loc.Title, "Somewhere")
	}

	if loc.Latitude != "" {
		t.Errorf("Latitude = %q, want empty", loc.Latitude)
	}

	if loc.Longitude != "" {
		t.Errorf("Longitude = %q, want empty", loc.Longitude)
	}
}

func TestCreatePageSorting(t *testing.T) {
	cal := ics.NewCalendar()
	now := time.Now()

	// Past events (added out of chronological order).
	addAllDayEvent(cal, "past-old", now.AddDate(0, -4, 0), now.AddDate(0, -4, 1), "Past Old")
	addAllDayEvent(cal, "past-recent", now.AddDate(0, -1, 0), now.AddDate(0, -1, 1), "Past Recent")

	// Current event (spans today).
	addAllDayEvent(cal, "current", now.AddDate(0, 0, -2), now.AddDate(0, 0, 2), "Current")

	// Future events (added out of chronological order, within monthsFuture=3 window).
	addAllDayEvent(cal, "future-far", now.AddDate(0, 2, 15), now.AddDate(0, 2, 16), "Future Far")
	addAllDayEvent(cal, "future-soon", now.AddDate(0, 1, 0), now.AddDate(0, 1, 1), "Future Soon")

	p, err := createPage(cal, t.Logf)
	if err != nil {
		t.Fatal(err)
	}

	// Bucketing.
	if len(p.Past) != 2 {
		t.Fatalf("Past: got %d events, want 2", len(p.Past))
	}

	if len(p.Future) != 2 {
		t.Fatalf("Future: got %d events, want 2", len(p.Future))
	}

	if p.Current == nil {
		t.Fatal("Current is nil, expected a current event")
	}

	if p.Current.Summary != "Current" {
		t.Errorf("Current.Summary = %q, want %q", p.Current.Summary, "Current")
	}

	// Past is reverse-sorted by To date (most recent first).
	if p.Past[0].Summary != "Past Recent" {
		t.Errorf("Past[0] = %q, want %q", p.Past[0].Summary, "Past Recent")
	}

	if p.Past[1].Summary != "Past Old" {
		t.Errorf("Past[1] = %q, want %q", p.Past[1].Summary, "Past Old")
	}

	// Future is sorted by To date (earliest first).
	if p.Future[0].Summary != "Future Soon" {
		t.Errorf("Future[0] = %q, want %q", p.Future[0].Summary, "Future Soon")
	}

	if p.Future[1].Summary != "Future Far" {
		t.Errorf("Future[1] = %q, want %q", p.Future[1].Summary, "Future Far")
	}
}

func TestCreatePageWithDescription(t *testing.T) {
	cal := ics.NewCalendar()
	now := time.Now()

	// Event with description (past).
	ev1 := cal.AddEvent("with-desc")
	ev1.SetAllDayStartAt(now.AddDate(0, -1, 0))
	ev1.SetAllDayEndAt(now.AddDate(0, -1, 1))
	ev1.SetSummary("With Desc")
	ev1.SetDescription("some description")

	// Event without description (past, older).
	ev2 := cal.AddEvent("without-desc")
	ev2.SetAllDayStartAt(now.AddDate(0, -2, 0))
	ev2.SetAllDayEndAt(now.AddDate(0, -2, 1))
	ev2.SetSummary("No Desc")

	p, err := createPage(cal, t.Logf)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Past) != 2 {
		t.Fatalf("expected 2 past events, got %d", len(p.Past))
	}

	// Past is reverse-sorted: "With Desc" (more recent) first.
	if p.Past[0].Summary != "With Desc" {
		t.Fatalf("Past[0] = %q, want 'With Desc'", p.Past[0].Summary)
	}

	if len(p.Past[0].Description) == 0 {
		t.Error("expected non-empty description for 'With Desc' event")
	}

	if p.Past[1].Summary != "No Desc" {
		t.Fatalf("Past[1] = %q, want 'No Desc'", p.Past[1].Summary)
	}

	if len(p.Past[1].Description) != 0 {
		t.Errorf("expected empty description for 'No Desc' event, got %v", p.Past[1].Description)
	}
}

func TestEventsClampTo(t *testing.T) {
	es := makePageEvents(3)

	// to=999 exceeds len(es)=3, should be clamped without panic.
	result := events(es, "future", 0, 999)
	if len(result) < 3 {
		t.Errorf("expected at least 3 results, got %d", len(result))
	}
}

func TestEventsEmptySlice(t *testing.T) {
	// nil slice should not panic.
	result := events(nil, "future", 0, 5)
	if result == nil {
		t.Error("expected non-nil result for nil input")
	}

	// Empty slice should not panic.
	result = events(pageEvents{}, "past", 0, 5)
	if result == nil {
		t.Error("expected non-nil result for empty input")
	}
}

func TestFetchCalendarValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, validICS)
	}))
	defer ts.Close()

	cal, err := fetchCalendar(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cal == nil {
		t.Fatal("expected non-nil calendar")
	}

	if len(cal.Events()) != 1 {
		t.Errorf("expected 1 event, got %d", len(cal.Events()))
	}
}

func TestFetchCalendarParseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "this is not valid ical data")
	}))
	defer ts.Close()

	_, err := fetchCalendar(ts.URL)
	if err == nil {
		t.Error("expected error for invalid ICS data")
	}
}

// ============================================================
// Group 2: Bug witnesses — assert correct behavior, skipped until fixed
// ============================================================

func TestParseTokensEmpty(t *testing.T) {

	toks := parseTokens("")
	if toks.isValid("") {
		t.Error("empty token should not be valid when no tokens are configured")
	}
}

func TestCreatePageNilSummary(t *testing.T) {

	cal := ics.NewCalendar()
	now := time.Now()

	// Event without SUMMARY — should not panic.
	ev := cal.AddEvent("no-summary")
	ev.SetAllDayStartAt(now.AddDate(0, -1, 0))
	ev.SetAllDayEndAt(now.AddDate(0, -1, 1))
	// Intentionally no SetSummary.

	p, err := createPage(cal, t.Logf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Fatal("expected non-nil page")
	}
}

func TestEventsNegativeFrom(t *testing.T) {

	es := makePageEvents(5)

	// Should not panic — from should be clamped to 0.
	result := events(es, "future", -1, 5)
	if len(result) < 1 {
		t.Error("expected non-empty result")
	}
}

func TestEventsFromExceedsLen(t *testing.T) {

	es := makePageEvents(3)

	// Should not panic — should return empty or clamped result.
	_ = events(es, "future", 999, 1000)
}

func TestEventsFromGreaterThanTo(t *testing.T) {

	es := makePageEvents(10)

	// Should not panic — should return empty or clamped result.
	_ = events(es, "future", 7, 5)
}

func TestPagerInvalidFromValidTo(t *testing.T) {

	r := httptest.NewRequest("GET", "/?from=abc&to=5", nil)
	w := httptest.NewRecorder()

	_, _, err := pager(w, r)
	if err == nil {
		t.Error("expected error for non-numeric 'from' parameter")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestPagerBothInvalid(t *testing.T) {

	r := httptest.NewRequest("GET", "/?from=abc&to=xyz", nil)
	w := httptest.NewRecorder()

	_, _, err := pager(w, r)
	if err == nil {
		t.Error("expected error")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	// Body should contain only the first error message, not both concatenated.
	body := w.Body.String()
	if body != "invalid from" {
		t.Errorf("expected body %q, got %q", "invalid from", body)
	}
}

func TestFetchCalendarStatusCode(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		// Return valid ICS so parsing succeeds if status isn't checked.
		fmt.Fprint(w, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n")
	}))
	defer ts.Close()

	_, err := fetchCalendar(ts.URL)
	if err == nil {
		t.Error("expected error for non-200 HTTP status code")
	}
}

func TestFetchCalendarBodyClosed(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, validICS)
	}))
	defer ts.Close()

	tracker := &trackingTransport{base: http.DefaultTransport}
	origTransport := httpClient.Transport
	httpClient.Transport = tracker

	defer func() { httpClient.Transport = origTransport }()

	_, err := fetchCalendar(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracker.bodies) != 1 {
		t.Fatalf("expected 1 tracked body, got %d", len(tracker.bodies))
	}

	if !tracker.bodies[0].closed {
		t.Error("response body was not closed after fetchCalendar")
	}
}

// ============================================================
// Group 3: Concurrency — documents the data race on hvor fields
// ============================================================

func TestHvorConcurrentAccess(t *testing.T) {
	// This test exercises the concurrent read/write pattern on hvor fields.
	// It passes without -race but will detect the data race under:
	//   go test -race ./...
	cal := ics.NewCalendar()
	now := time.Now()
	addAllDayEvent(cal, "e1", now.AddDate(0, 0, -2), now.AddDate(0, 0, 2), "Current")
	addAllDayEvent(cal, "e2", now.AddDate(0, -1, 0), now.AddDate(0, -1, 1), "Past")
	addAllDayEvent(cal, "e3", now.AddDate(0, 1, 0), now.AddDate(0, 1, 1), "Future")

	p, err := createPage(cal, t.Logf)
	if err != nil {
		t.Fatal(err)
	}

	h := &hvor{
		logf: t.Logf,
	}
	h.snap.Store(&snapshot{calPage: p, lastFetch: time.Now()})

	var wg sync.WaitGroup

	// Writer goroutine (simulates the background updater).
	wg.Add(1)

	go func() {
		defer wg.Done()

		for range 100 {
			newPage := &page{
				Past:   make(pageEvents, 0),
				Future: make(pageEvents, 0),
			}
			h.snap.Store(&snapshot{calPage: newPage, lastFetch: time.Now()})
		}
	}()

	// Reader goroutines (simulate concurrent HTTP handlers).
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100 {
				s := h.snap.Load()
				if s != nil {
					_ = s.lastFetch

					if s.calPage != nil {
						_ = s.calPage.Past
						_ = s.calPage.Future
						_ = s.calPage.Current
					}
				}
			}
		}()
	}

	wg.Wait()
}
