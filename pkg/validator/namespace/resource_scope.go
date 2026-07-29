package namespace

var resourceScope = map[string]bool{
	"/Namespace": true,
	"rbac.authorization.k8s.io/ClusterRole": true,
	"cli.kyverno.io/Test": true,
	"cli.kyverno.io/Values": true,
	"apps/Deployment": false,
	"/Service": false,
	"/ConfigMap": false,
	"/Secret": false,
}
