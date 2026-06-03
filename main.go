package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintln(os.Stderr, `cyclone2 — GameSir Cyclone 2 battery tool

usage:
  cyclone2 probe     reverse-engineering helper (dumps HID frames)
  cyclone2 status    print battery percentage once (--json for machine output)
  cyclone2 daemon    poll battery and write the state file for the GNOME extension`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "probe":
		err = runProbe(os.Args[2:])
	case "status":
		err = runStatus(os.Args[2:])
	case "daemon":
		err = runDaemon(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
