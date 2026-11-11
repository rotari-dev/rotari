package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rotari-dev/rotari/internal/project"
	"github.com/rotari-dev/rotari/internal/state"
)

func showAllProjects(cliBaseDir, cliMasterDir string) int {
	baseDirs, missing, err := jobsBaseDirs(cliBaseDir, cliMasterDir)
	if err != nil {
		printError(err)
		return 1
	}
	warnMissingBaseDirs(missing)
	return showProjectsForBaseDirs(baseDirs)
}

func showProjectsForBaseDirs(baseDirs []string) int {
	fmt.Printf("%s\n", cyan("=== PROJECTS ==="))

	type projectInfo struct {
		baseDir    string
		name       string
		queued     int
		runs       int
		state      string
		lastRun    string
		lastResult string
	}
	projects := make([]projectInfo, 0)
	for _, baseDir := range baseDirs {
		overviews, err := project.Overviews(baseDir)
		if errors.Is(err, state.ErrBaseDirMissing) {
			// The default basedir before its first run.
			continue
		}
		if err != nil {
			printError(err)
			return 1
		}
		for _, overview := range overviews {
			lastRun := firstNonEmpty(overview.LastRun.ID, "-")
			projects = append(projects, projectInfo{baseDir: baseDir, name: overview.Name, queued: overview.Queued, runs: overview.Runs,
				state: projectStateName(overview.State), lastRun: lastRun, lastResult: lastRunResult(overview)})
		}
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return 0
	}
	fmt.Printf("\n%s\n", cyan(fmt.Sprintf("Projects: %d", len(projects))))
	table := textTable{}
	for _, header := range []string{"BASEDIR", "PROJECT", "QUEUED", "RUNS", "STATE", "LAST RUN", "LAST RESULT"} {
		table.columns = append(table.columns, tableColumn{header: header})
	}
	for _, project := range projects {
		table.rows = append(table.rows, []string{project.baseDir, project.name, strconv.Itoa(project.queued), strconv.Itoa(project.runs), project.state, project.lastRun, project.lastResult})
	}
	table.write(os.Stdout)
	// Without -b, `show -p` resolves the default state directory, so the
	// hint names the basedir when a listed project lives elsewhere.
	defaultBaseDir, _, defaultErr := state.ResolveBaseDir("")
	otherBaseDir, hasRun := false, false
	for _, project := range projects {
		otherBaseDir = otherBaseDir || defaultErr != nil || filepath.Clean(project.baseDir) != filepath.Clean(defaultBaseDir)
		hasRun = hasRun || project.lastRun != "-"
	}
	fmt.Println("\n" + cyan("To inspect a project:"))
	if otherBaseDir {
		fmt.Println("  rotari show -b BASEDIR -p PROJECT")
	} else {
		fmt.Println("  rotari show -p PROJECT")
	}
	if hasRun {
		fmt.Println(cyan("To summarize a run's failures by cause:"))
		fmt.Println("  rotari lineage RUN_ID")
	}
	return 0
}

// lastRunResult describes a project's last run for the project list:
// "running" while it runs, its status, with failed and total job counts once
// any job failed, or "-" without a readable summary.
func lastRunResult(overview project.Overview) string {
	last := overview.LastRun
	switch {
	case overview.State == project.Running:
		return "running"
	case last.Status == "":
		return "-"
	case last.Failed == 0:
		return last.Status
	}
	return fmt.Sprintf("%s %d/%d", last.Status, last.Failed, last.Jobs)
}
