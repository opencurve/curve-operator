package curve

import (
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:    "curve",
	Short:  "Curve (curve.io) Kubernetes operator and user tools",
	Hidden: false,
}
