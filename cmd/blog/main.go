package main

import (
	"flag"
	"fmt"
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
	flags.String("config", "config.yaml", "configuration file")
	if command == "build" {
		flags.String("output", "", "independent output directory (required)")
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
	// Command implementations are added at their implementation-plan gates.
	fmt.Fprintf(errOut, "blog %s: not implemented\n", command)
	return 1
}
