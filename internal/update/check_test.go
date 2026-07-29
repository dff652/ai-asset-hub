package update

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckReportsReleaseRelationship(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		wantState string
		wantNew   bool
	}{
		{name: "older", current: "0.1.2", wantState: StatusUpdateAvailable, wantNew: true},
		{name: "current", current: "0.1.3", wantState: StatusCurrent},
		{name: "ahead", current: "0.2.0", wantState: StatusAhead},
		{name: "development", current: "dev", wantState: StatusDevelopment},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var userAgent, accept string
			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				userAgent = request.Header.Get("User-Agent")
				accept = request.Header.Get("Accept")
				_, _ = writer.Write([]byte(`{
					"tag_name":"v0.1.3",
					"html_url":"https://github.example/releases/tag/v0.1.3"
				}`))
			}))
			t.Cleanup(server.Close)

			report, err := Check(Options{
				CurrentVersion: test.current,
				Endpoint:       server.URL,
				Client:         server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if !report.Ok || report.CurrentVersion != test.current ||
				report.LatestVersion != "0.1.3" ||
				report.Status != test.wantState ||
				report.UpdateAvailable != test.wantNew ||
				report.ReleaseURL != "https://github.example/releases/tag/v0.1.3" ||
				report.UpgradeCommand != upgradeCommandFor("0.1.3") {
				t.Fatalf("report = %#v", report)
			}
			if !strings.HasPrefix(userAgent, "aiah/") ||
				accept != "application/vnd.github+json" {
				t.Fatalf("headers: user-agent=%q accept=%q", userAgent, accept)
			}
		})
	}
}

func TestCheckRejectsInvalidReleaseResponse(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "http error", status: http.StatusBadGateway, body: `{}`},
		{name: "invalid json", status: http.StatusOK, body: `{`},
		{name: "invalid tag", status: http.StatusOK, body: `{"tag_name":"latest"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				_ *http.Request,
			) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			report, err := Check(Options{
				CurrentVersion: "0.1.2",
				Endpoint:       server.URL,
				Client:         server.Client(),
			})
			if err == nil || report.Ok {
				t.Fatalf("err=%v report=%#v", err, report)
			}
		})
	}
}

func TestCheckRejectsOversizedOtherwiseValidResponse(t *testing.T) {
	padding := strings.Repeat(" ", maxReleaseResponseBytes)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = writer.Write([]byte(`{
			"tag_name":"v0.1.3",
			"html_url":"https://github.example/releases/tag/v0.1.3"
		}` + padding))
	}))
	t.Cleanup(server.Close)

	report, err := Check(Options{
		CurrentVersion: "0.1.2",
		Endpoint:       server.URL,
		Client:         server.Client(),
	})
	if err == nil || report.Ok {
		t.Fatalf("oversized response accepted: err=%v report=%#v", err, report)
	}
}

func TestCompareStableVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{left: "0.1.2", right: "0.1.3", want: -1},
		{left: "0.1.3", right: "0.1.3", want: 0},
		{left: "1.0.0", right: "0.99.99", want: 1},
	}
	for _, test := range tests {
		got, ok := compareStableVersions(test.left, test.right)
		if !ok || got != test.want {
			t.Fatalf("compare(%q, %q) = %d, %v; want %d, true",
				test.left, test.right, got, ok, test.want)
		}
	}
	if _, ok := compareStableVersions("dev", "0.1.3"); ok {
		t.Fatal("development version was treated as a stable release")
	}
}
