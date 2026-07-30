package pipeline

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
)

// Options configures the pipeline run.
//
// Step/check enablement uses one generic ID-based mechanism instead of
// dedicated boolean flags per step - see the doc comment on
// validator.Options for the full explanation. DisabledChecks/EnabledChecks
// are passed straight through to validator.Options unchanged.
type Options struct {
	URL             string
	PR              string
	Revision        string
	TargetBranch    string
	HookSource      string
	TriggerComment  string
	LintOnly        bool
	NoComment       bool
	Verbose         bool
	AssumeOpenShift bool     // treat OpenShift/OKD-only API groups as exempt from the sync-options check
	DisabledChecks  []string // IDs to disable entirely (e.g. "sync-options", "golangci", "avp"); only affects steps that default to enabled
	EnabledChecks   []string // IDs to explicitly enable; only affects steps that default to disabled (e.g. "kyverno")
	Concurrency     int
	Apps            []string
	Clusters        []string
	IncludePrefixes []string // restrict the changeset to files under these path prefixes (e.g. "kubernetes/", "tekton/"); empty means no restriction
	Providers       provider.Providers
}

// Result captures the pipeline outcome.
type Result struct {
	PRValid          bool
	TitleErr         error
	UnsignedErr      error
	ChecklistErr     error
	ValidatorResult  *validator.Result
	ValidationErr    error
	ReproduceCommand string
}

// Run executes the pipeline phases.
func Run(opts Options) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = resolveRevision(opts.URL)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = opts.Providers.ReportMarker()
	}()
	wg.Wait()

	res := &Result{}
	if isValidPR(opts.PR) && !opts.isMergeQueue() && !opts.LintOnly {
		client := github.NewClient(opts.URL, opts.PR)
		res.TitleErr = github.ValidatePRTitle(client)
		res.UnsignedErr = runUnsignedCheck(client)
		res.ChecklistErr = github.ValidatePRChecklist(client)
	}
	if opts.LintOnly {
		res.PRValid = true
	}

	if shouldRunValidation(opts) {
		vopts := toValidatorOptions(opts)
		vr, err := validator.RunAll(vopts)
		res.ValidatorResult = vr
		res.ValidationErr = err
	}

	res.ReproduceCommand = validator.ReproduceCommand(toValidatorOptions(opts))
	if !opts.NoComment {
		_ = postComment(res, opts)
	}
	if res.ValidationErr != nil || res.TitleErr != nil || res.UnsignedErr != nil {
		return fmt.Errorf("pipeline completed with failures")
	}
	return nil
}

func resolveRevision(raw string) string {
	if raw == "" {
		return "HEAD"
	}
	return raw
}

func isValidPR(pr string) bool {
	if pr == "" {
		return false
	}
	matched, _ := regexp.MatchString(`\{\{.*\}\}`, pr)
	return !matched
}

func (o *Options) isMergeQueue() bool {
	return strings.Contains(o.TargetBranch, "gh-readonly-queue/")
}

func (o *Options) Workers() int {
	if o.Concurrency > 0 {
		return o.Concurrency
	}
	return runtime.NumCPU() * 2
}

func shouldRunValidation(opts Options) bool {
	return true
}

func toValidatorOptions(opts Options) validator.Options {
	return validator.Options{
		RepoURL:         opts.URL,
		PR:              opts.PR,
		BaseRef:         resolveBaseRef(opts.TargetBranch),
		Revision:        resolveRevision(opts.Revision),
		LintOnly:        opts.LintOnly,
		NoComment:       opts.NoComment,
		AssumeOpenShift: opts.AssumeOpenShift,
		DisabledChecks:  opts.DisabledChecks,
		EnabledChecks:   opts.EnabledChecks,
		Concurrency:     opts.Workers(),
		Apps:            opts.Apps,
		Clusters:        opts.Clusters,
		IncludePrefixes: opts.IncludePrefixes,
		Providers:       opts.Providers,
	}
}

func resolveBaseRef(targetBranch string) string {
	if strings.HasPrefix(targetBranch, "gh-readonly-queue/") {
		parts := strings.Split(targetBranch, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	if targetBranch != "" {
		return targetBranch
	}
	return "origin/main"
}

func runUnsignedCheck(c *github.Client) error {
	commits, err := github.GetUnsignedCommits(c)
	if err != nil {
		return err
	}
	if len(commits) > 0 {
		return fmt.Errorf("%d unsigned commits detected", len(commits))
	}
	return nil
}

func postComment(res *Result, opts Options) error {
	client := github.NewClient(opts.URL, opts.PR)
	if !client.IsAvailable() {
		return nil
	}
	marker := opts.Providers.ReportMarker()
	if marker == "" {
		marker = "<!-- gitops-ci-report -->"
	}
	report := validator.Report{
		Marker:   marker,
		Title:    "GitOps CI Results",
		Header:   "GitOps CI Pipeline",
		Sections: composeSections(res, opts),
		Body:     "```bash\n" + res.ReproduceCommand + "\n```",
	}
	if err := github.UpsertComment(client, marker, report.Render()); err != nil {
		return err
	}
	return github.DeleteComments(client, validator.LegacyMarkers()...)
}

func composeSections(res *Result, opts Options) []validator.Section {
	var sections []validator.Section

	// 1. PR Checks
	sections = append(sections, validator.ComposePRChecksSection(res.TitleErr, res.UnsignedErr, res.ChecklistErr))

	// 2–3. Linting + Static Checks pulled from ValidatorResult sections if present.
	lintReports := map[string]string{}
	staticReports := map[string]string{}
	if res.ValidatorResult != nil {
		for _, s := range res.ValidatorResult.Sections {
			switch s.Name {
			case "Linting":
				lintReports["summary"] = s.Body
			case "Static Checks":
				staticReports["summary"] = s.Body
			}
		}
	}
	sections = append(sections, validator.ComposeLintingSection(lintReports))
	sections = append(sections, validator.ComposeStaticChecksSection(staticReports))

	// 4. Kustomize Build
	sections = append(sections, validator.ComposeKustomizeBuildSection(0, nil, nil, nil))

	// 5. Scaffold Validation
	sections = append(sections, validator.ComposeScaffoldValidationSection("", nil, nil))

	// 6. Resource Compliance
	if res.ValidatorResult != nil {
		sections = append(sections, validator.ComposeResourceComplianceSection(res.ValidatorResult.Check.Findings))
	} else {
		sections = append(sections, validator.Section{Name: "Resource Compliance", Body: "No results."})
	}

	// 7. CI Notes
	_ = opts
	sections = append(sections, validator.ComposeCINotesSection("Pipeline completed."))
	return sections
}

// EnvOptions loads options from environment.
func EnvOptions() Options {
	return Options{
		URL:            os.Getenv("PARAMS_URL"),
		PR:             os.Getenv("PARAMS_PR"),
		Revision:       os.Getenv("PARAMS_REVISION"),
		TargetBranch:   os.Getenv("PARAMS_TARGET_BRANCH"),
		TriggerComment: os.Getenv("PARAMS_TRIGGER_COMMENT"),
	}
}
