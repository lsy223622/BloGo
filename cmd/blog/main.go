package main

import (
	"flag"
	"fmt"
	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"io"
	"os"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	const usage = "Usage: blog <build|publish|watch|dev|stats|report> [options]\n"
	if len(args) == 0 {
		fmt.Fprint(errOut, usage)
		return 1
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, usage)
		return 0
	}
	command := args[0]
	switch command {
	case "build", "publish", "watch", "dev", "stats", "report":
	default:
		fmt.Fprintf(errOut, "blog: unknown command %q\n%s", command, usage)
		return 1
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	configFile := flags.String("config", "config.yaml", "configuration file")
	var output string
	if command == "build" {
		flags.StringVar(&output, "output", "", "independent output directory (required)")
	}
	if command == "dev" {
		flags.StringVar(&output, "output", "output/dev", "independent preview output directory")
	}
	flags.Usage = func() {
		fmt.Fprintf(out, "Usage: blog %s [options]\n", command)
		flags.SetOutput(out)
		flags.PrintDefaults()
		flags.SetOutput(errOut)
	}
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "blog: unexpected positional arguments")
		return 1
	}
	c, err := config.Load(*configFile)
	if err == nil && command == "build" {
		result, buildErr := build.Build(c, output, false)
		if buildErr != nil {
			fmt.Fprintf(errOut, "blog build: %v\n", buildErr)
			return 1
		}
		for _, warning := range result.Warnings {
			fmt.Fprintf(errOut, "warning: %s\n", warning)
		}
		fmt.Fprintf(out, "Built %d pages in %s (%s)\n", len(result.Manifest.Pages), result.Output, result.Manifest.BuildID)
		return 0
	}
	if err == nil && command == "publish" {
		result, publishErr := build.Publish(c)
		if publishErr != nil {
			fmt.Fprintf(errOut, "blog publish: %v\n", publishErr)
			return 1
		}
		for _, warning := range result.Warnings {
			fmt.Fprintf(errOut, "warning: %s\n", warning)
		}
		fmt.Fprintf(out, "Published %s\n", result.Output)
		return 0
	}
	if err == nil {
		err = c.Validate(command, output)
	}
	if err != nil {
		fmt.Fprintf(errOut, "blog %s: %v\n", command, err)
		return 1
	}
	fmt.Fprintf(errOut, "blog %s: not implemented\n", command)
	return 1
}
