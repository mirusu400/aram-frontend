package frontend

import (
	runtimedebug "runtime/debug"
	"slices"
	"strings"
)

type debugBuildReport struct {
	Main       debugModuleReport   `json:"main"`
	Components []debugModuleReport `json:"components,omitempty"`
	VCS        string              `json:"vcs,omitempty"`
	Revision   string              `json:"revision,omitempty"`
	Time       string              `json:"time,omitempty"`
	Modified   bool                `json:"modified,omitempty"`
}

type debugModuleReport struct {
	Path        string             `json:"path"`
	Version     string             `json:"version,omitempty"`
	Sum         string             `json:"sum,omitempty"`
	Replacement *debugModuleReport `json:"replacement,omitempty"`
}

func currentDebugBuildReport() debugBuildReport {
	info, ok := runtimedebug.ReadBuildInfo()
	if !ok {
		return debugBuildReport{}
	}
	report := debugBuildReport{Main: debugModule(info.Main)}
	for _, dependency := range info.Deps {
		if dependency == nil ||
			dependency.Path != "github.com/mirusu400/aram-core" &&
				dependency.Path != "github.com/mirusu400/aram-frontend" {
			continue
		}
		report.Components = append(
			report.Components,
			debugModule(*dependency),
		)
	}
	slices.SortFunc(
		report.Components,
		func(left, right debugModuleReport) int {
			return strings.Compare(left.Path, right.Path)
		},
	)
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs":
			report.VCS = setting.Value
		case "vcs.revision":
			report.Revision = setting.Value
		case "vcs.time":
			report.Time = setting.Value
		case "vcs.modified":
			report.Modified = setting.Value == "true"
		}
	}
	return report
}

func debugModule(module runtimedebug.Module) debugModuleReport {
	report := debugModuleReport{
		Path:    module.Path,
		Version: module.Version,
		Sum:     module.Sum,
	}
	if module.Replace != nil {
		replacement := debugModule(*module.Replace)
		report.Replacement = &replacement
	}
	return report
}

func redactDebugBuildReport(report *debugBuildReport, roots []string) {
	redactDebugModule(&report.Main, roots)
	for index := range report.Components {
		redactDebugModule(&report.Components[index], roots)
	}
}

func redactDebugModule(module *debugModuleReport, roots []string) {
	if module == nil {
		return
	}
	module.Path = redactDebugText(module.Path, roots)
	redactDebugModule(module.Replacement, roots)
}
