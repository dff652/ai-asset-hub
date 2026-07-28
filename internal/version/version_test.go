package version

import "testing"

func TestProducedByAndFull(t *testing.T) {
	original := []string{Version, Commit, Date}
	t.Cleanup(func() { Version, Commit, Date = original[0], original[1], original[2] })

	tests := []struct {
		name           string
		version        string
		commit         string
		date           string
		wantProducedBy string
		wantFull       string
	}{
		{
			name: "unstamped dev build", version: "dev",
			wantProducedBy: "aiah dev", wantFull: "aiah dev",
		},
		{
			name: "release build", version: "0.1.0",
			commit: "d9dd3b263a6f8e01ab", date: "2026-07-25T13:26:17Z",
			wantProducedBy: "aiah 0.1.0+d9dd3b263a6f",
			wantFull:       "aiah 0.1.0, commit d9dd3b263a6f, built 2026-07-25T13:26:17Z",
		},
		{
			name: "short commit is not truncated", version: "0.1.0", commit: "abc123",
			wantProducedBy: "aiah 0.1.0+abc123",
			wantFull:       "aiah 0.1.0, commit abc123",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Version, Commit, Date = test.version, test.commit, test.date
			if got := ProducedBy(); got != test.wantProducedBy {
				t.Errorf("ProducedBy() = %q, want %q", got, test.wantProducedBy)
			}
			if got := Full(); got != test.wantFull {
				t.Errorf("Full() = %q, want %q", got, test.wantFull)
			}
		})
	}
}
