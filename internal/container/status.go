package container

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// FormatStatus writes a human-readable status table for a container to w.
func FormatStatus(w io.Writer, info containertypes.InspectResponse) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Container:\t%s\n", strings.TrimPrefix(info.Name, "/"))
	fmt.Fprintf(tw, "State:\t%s\n", info.State.Status)
	fmt.Fprintf(tw, "Image:\t%s\n", info.Config.Image)

	if info.State.Running {
		start, err := time.Parse(time.RFC3339Nano, info.State.StartedAt)
		if err == nil {
			fmt.Fprintf(tw, "Uptime:\t%s\n", time.Since(start).Truncate(time.Second))
		}
	} else if info.State.FinishedAt != "" && info.State.FinishedAt != "0001-01-01T00:00:00Z" {
		fmt.Fprintf(tw, "Exited at:\t%s\n", info.State.FinishedAt)
	}

	for _, env := range info.Config.Env {
		if strings.HasPrefix(env, "CLAUSTRO_HOST_PATH=") {
			fmt.Fprintf(tw, "Host path:\t%s\n", strings.TrimPrefix(env, "CLAUSTRO_HOST_PATH="))
			break
		}
	}

	fmt.Fprintln(tw, "\nMounts:")
	for _, m := range info.Mounts {
		fmt.Fprintf(tw, "  %s\t→\t%s\n", m.Source, m.Destination)
	}

	fmt.Fprintln(tw, "\nNetworks:")
	for name := range info.NetworkSettings.Networks {
		fmt.Fprintf(tw, "  %s\n", name)
	}

	return tw.Flush()
}
