// Package migration builds a read-only cross-device status from the same
// workspace, installation, and channel Core reports used by the CLI.
package migration

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type Options struct {
	WorkspaceRoot string
	ManifestPath  string
	Channel       string
	Home          string
	Project       string
}

type LibraryStatus struct {
	Root       string   `json:"root"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	AssetCount int      `json:"assetCount"`
	Profiles   []string `json:"profiles"`
	Ok         bool     `json:"ok"`
}

type InstallationStatus struct {
	Present  bool     `json:"present"`
	Ok       bool     `json:"ok"`
	Package  string   `json:"package,omitempty"`
	Version  string   `json:"version,omitempty"`
	Profile  string   `json:"profile,omitempty"`
	Targets  []string `json:"targets,omitempty"`
	BackupID string   `json:"backupId,omitempty"`
}

type ChannelStatus struct {
	Selected     bool             `json:"selected"`
	Path         string           `json:"path,omitempty"`
	ReleaseCount int              `json:"releaseCount"`
	Latest       *channel.Release `json:"latest,omitempty"`
}

type Alignment struct {
	Installation string `json:"installation"`
	Channel      string `json:"channel"`
}

type Report struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Kind          string              `json:"kind"`
	Ok            bool                `json:"ok"`
	Library       LibraryStatus       `json:"library"`
	Installation  InstallationStatus  `json:"installation"`
	Channel       ChannelStatus       `json:"channel"`
	Alignment     Alignment           `json:"alignment"`
	Findings      []workspace.Finding `json:"findings"`
}

// Inspect reports local migration readiness without publishing, pulling,
// building, applying, or writing any state.
func Inspect(options Options) (Report, error) {
	report := Report{
		SchemaVersion: 1,
		Kind:          "migration-status",
		Channel:       ChannelStatus{},
		Alignment: Alignment{
			Installation: "not-installed",
			Channel:      "channel-not-selected",
		},
		Findings: []workspace.Finding{},
	}
	manifestPath := strings.TrimSpace(options.ManifestPath)
	if manifestPath == "" {
		manifestPath = filepath.Join(options.WorkspaceRoot, "manifest.yaml")
	}
	inspection, err := workspace.Inspect(workspace.Options{
		Manifest: manifestPath,
		Root:     options.WorkspaceRoot,
	})
	if err != nil {
		return report, err
	}
	profiles := make([]string, 0, len(inspection.Document.Profiles))
	for name := range inspection.Document.Profiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)
	report.Library = LibraryStatus{
		Root:       inspection.Root,
		Name:       inspection.Document.Name,
		Version:    inspection.Document.Version,
		AssetCount: len(inspection.Document.Assets),
		Profiles:   profiles,
		Ok:         !workspace.HasError(inspection.Findings),
	}
	report.Findings = append(report.Findings, inspection.Findings...)

	doctor, err := apply.Doctor(apply.DoctorOptions{
		Home: options.Home, Project: options.Project,
	})
	if err != nil {
		return report, err
	}
	report.Installation.Ok = doctor.Ok
	if doctor.Deployment != nil {
		report.Installation = InstallationStatus{
			Present:  true,
			Ok:       doctor.Ok,
			Package:  doctor.Deployment.Package,
			Version:  doctor.Deployment.Version,
			Profile:  doctor.Deployment.Profile,
			Targets:  append([]string(nil), doctor.Deployment.Targets...),
			BackupID: doctor.Deployment.BackupID,
		}
		report.Alignment.Installation = compareInstallation(
			report.Library, report.Installation,
		)
	}
	report.Findings = append(report.Findings, doctor.Findings...)

	if strings.TrimSpace(options.Channel) != "" {
		list, listErr := channel.List(channel.ListOptions{
			Channel: options.Channel,
			Name:    report.Library.Name,
		})
		if listErr != nil {
			return report, listErr
		}
		report.Channel.Selected = true
		report.Channel.Path = list.Channel
		report.Channel.ReleaseCount = len(list.Releases)
		if len(list.Releases) > 0 {
			latest := list.Releases[len(list.Releases)-1]
			report.Channel.Latest = &latest
		}
		report.Alignment.Channel = compareChannel(report.Library, report.Channel)
	}

	report.Ok = report.Library.Ok && report.Installation.Ok
	return report, nil
}

func compareInstallation(library LibraryStatus, installation InstallationStatus) string {
	if !installation.Present {
		return "not-installed"
	}
	if installation.Package != library.Name {
		return "different-package"
	}
	if installation.Version == library.Version {
		return "same-version"
	}
	return "different-version"
}

func compareChannel(library LibraryStatus, status ChannelStatus) string {
	if !status.Selected {
		return "channel-not-selected"
	}
	if status.Latest == nil {
		return "not-published"
	}
	if status.Latest.Version == library.Version {
		return "same-version"
	}
	return "different-version"
}
