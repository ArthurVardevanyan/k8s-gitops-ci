package main

import (
	"flag"
	"fmt"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/cireport"
)

// runCIReport posts or updates a self-CI status comment on this repo's own PR.
//
// It is a thin flag-parsing shim over pkg/cireport (the org-agnostic builder +
// poster), invoked from the meta-CI pipeline (.tekton/k8s-gitops-ci.yaml's
// build task) AFTER `task ci` and the live regression replay have run, with
// their outcomes passed in via flags. The comment always reflects the blocking
// task-ci verdict, plus a non-blocking, informational replay section.
//
// This command NEVER fails the build itself: a missing PR context, an
// unavailable GitHub client, or a comment-post error is reported but returns
// nil, because the authoritative pass/fail gate is task ci's own exit code,
// re-asserted by the calling pipeline step — not this reporter.
func runCIReport(args []string) error {
	fs := flag.NewFlagSet("ci-report", flag.ExitOnError)
	var (
		url          = fs.String("url", "", "repository URL (e.g. https://github.com/org/repo)")
		pr           = fs.String("pr", "", "pull request number")
		ciStatus     = fs.String("ci-status", "", "overall `task ci` result: pass|fail")
		ciReport     = fs.String("ci-report", "", "path to a file with `task ci` failure detail (optional; embedded when ci-status=fail)")
		replayStatus = fs.String("replay-status", "skipped", "live replay result: pass|warn|fail|skipped")
		replayReport = fs.String("replay-report", "", "path to the replay's Markdown report (optional)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	msg, err := cireport.Run(cireport.Options{
		URL:          *url,
		PR:           *pr,
		CIStatus:     *ciStatus,
		CIDetail:     cireport.ReadDetailFile(*ciReport),
		ReplayStatus: *replayStatus,
		ReplayReport: cireport.ReadDetailFile(*replayReport),
		ReplayLabel:  "HomeLab",
		DocsURL:      "https://github.com/ArthurVardevanyan/k8s-gitops-ci/blob/main/docs/DEVELOPMENT.md#end-to-end--regression-replay",
	})
	if err != nil {
		return err
	}
	fmt.Println("ci-report: " + msg)
	return nil
}
