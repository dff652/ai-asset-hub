// Package update checks the latest supported aiah release without changing
// the installed binary.
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dff652/ai-asset-hub/internal/version"
)

const (
	LatestReleaseEndpoint = "https://api.github.com/repos/dff652/ai-asset-hub/releases/latest"

	StatusCurrent         = "current"
	StatusUpdateAvailable = "update-available"
	StatusAhead           = "ahead"
	StatusDevelopment     = "development"

	maxReleaseResponseBytes = 1 << 20
)

var stableVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Options struct {
	CurrentVersion string
	Endpoint       string
	Client         HTTPClient
}

type Report struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Kind            string `json:"kind"`
	ProducedBy      string `json:"producedBy"`
	Ok              bool   `json:"ok"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	Status          string `json:"status"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	UpgradeCommand  string `json:"upgradeCommand"`
}

type latestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func Check(options Options) (Report, error) {
	current := options.CurrentVersion
	if current == "" {
		current = version.Version
	}
	report := Report{
		SchemaVersion:  1,
		Kind:           "update-check",
		ProducedBy:     version.ProducedBy(),
		CurrentVersion: current,
	}

	endpoint := options.Endpoint
	if endpoint == "" {
		endpoint = LatestReleaseEndpoint
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return report, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "aiah/"+version.Version)

	response, err := client.Do(request)
	if err != nil {
		return report, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return report, fmt.Errorf("latest release request returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseResponseBytes+1))
	if err != nil {
		return report, err
	}
	if len(body) > maxReleaseResponseBytes {
		return report, errors.New("latest release response is too large")
	}

	var release latestRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return report, errors.New("latest release response is invalid")
	}
	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if !stableVersionPattern.MatchString(latest) {
		return report, errors.New("latest release tag is invalid")
	}
	if release.HTMLURL != "" {
		releaseURL, parseErr := url.Parse(release.HTMLURL)
		if parseErr != nil || releaseURL.Scheme == "" || releaseURL.Host == "" {
			return report, errors.New("latest release URL is invalid")
		}
	}

	report.LatestVersion = latest
	report.ReleaseURL = release.HTMLURL
	report.UpgradeCommand = upgradeCommandFor(latest)
	comparison, comparable := compareStableVersions(current, latest)
	switch {
	case !comparable:
		report.Status = StatusDevelopment
	case comparison < 0:
		report.Status = StatusUpdateAvailable
		report.UpdateAvailable = true
	case comparison > 0:
		report.Status = StatusAhead
	default:
		report.Status = StatusCurrent
	}
	report.Ok = true
	return report, nil
}

func upgradeCommandFor(release string) string {
	return fmt.Sprintf(
		"curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v%s/scripts/install.sh | AIAH_VERSION=%s sh",
		release,
		release,
	)
}

func compareStableVersions(left, right string) (int, bool) {
	if !stableVersionPattern.MatchString(left) ||
		!stableVersionPattern.MatchString(right) {
		return 0, false
	}
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	for index := range leftParts {
		leftValue, leftErr := strconv.ParseUint(leftParts[index], 10, 64)
		rightValue, rightErr := strconv.ParseUint(rightParts[index], 10, 64)
		if leftErr != nil || rightErr != nil {
			return 0, false
		}
		switch {
		case leftValue < rightValue:
			return -1, true
		case leftValue > rightValue:
			return 1, true
		}
	}
	return 0, true
}
