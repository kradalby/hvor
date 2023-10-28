package main

import "testing"

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
			t.Errorf("unexpected sanatised title, got: %s, want: %s", got, tt.want)
		}
	}
}
