package main

import (
	"fmt"
	"github.com/opencurve/curve-operator/cmd/curve"
)

func main() {
	addCommands()
	if err := curve.RootCmd.Execute(); err != nil {
		fmt.Printf("curve error: %+v\n", err)
	}
}

func addCommands() {
	curve.RootCmd.AddCommand(
		curve.OperatorCmd,
		curve.DiscoverCmd,
	)
}
