package main

import (
	"fmt"
	"os"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/pipeline"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	opts := pipeline.EnvOptions()
	opts.Providers = provider.Providers{}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version.String())
	case "pipeline", "ci":
		if err := pipeline.Run(opts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: gitops-ci <command> [flags]

Commands:
  pipeline      Run the CI pipeline
  version       Show version
  --help        Show this help`)
}
