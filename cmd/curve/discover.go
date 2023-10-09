package curve

import (
	"context"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/discover"
	"github.com/spf13/cobra"
	"k8s.io/klog"
	"os"
	"time"
)

var (
	DiscoverCmd = &cobra.Command{
		Use:    "discover",
		Short:  "Discover devices",
		Hidden: true, // do not advertise to end users
	}

	// interval between discovering devices
	discoverDevicesInterval time.Duration
)

func init() {
	DiscoverCmd.Flags().DurationVar(&discoverDevicesInterval, "discover-interval", 60*time.Minute,
		"interval between discovering devices (default 60m)")
	DiscoverCmd.Run = startDiscover
}

func startDiscover(cmd *cobra.Command, args []string) {
	clusterdContext := clusterd.NewContext()
	err := discover.Run(context.TODO(), clusterdContext, discoverDevicesInterval)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
