package validator

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

// appBuildStrategy pairs the Strategy an app's overlays should be rendered
// with and the set of overlay names excluded from AVP substitution (an
// app's test.sh AVP_EXCLUDE= - see hook.Config.AVPExclude).
type appBuildStrategy struct {
	Strategy overlay.Strategy
	Exclude  map[string]bool
}

// resolveAppBuildStrategies determines each app's build Strategy once, up
// front - alongside resolveAppHookConfigs, since both are per-app,
// resolved-once inputs to every one of that app's overlay builds (see
// buildOverlayWithHooks).
func resolveAppBuildStrategies(apps []string, avpEnabled bool, cfgs map[string]*hook.Config) map[string]appBuildStrategy {
	out := make(map[string]appBuildStrategy, len(apps))
	for _, app := range apps {
		abs := appBuildStrategy{Strategy: overlay.DetectStrategy(app, avpEnabled)}
		if cfg := cfgs[app]; cfg != nil {
			abs.Exclude = overlay.ExcludeSet(cfg.AVPExclude)
		}
		out[app] = abs
	}
	return out
}
