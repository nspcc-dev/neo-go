// Package sdnotify implements sd_notify(3) protocol for notifying systemd
// about service state changes.
package sdnotify

import (
	"net"
	"os"
)

// State messages for sd_notify.
const (
	// Ready tells systemd that the service is fully started and ready to
	// handle requests.
	Ready = "READY=1"
	// Reloading tells systemd that the service is reloading its configuration.
	// After completion the service should send [Ready] again.
	Reloading = "RELOADING=1"
	// Stopping tells systemd that the service is beginning its shutdown.
	Stopping = "STOPPING=1"
)

// Send sends a notification to systemd via the NOTIFY_SOCKET. If the
// NOTIFY_SOCKET environment variable is not set, Send is a no-op and
// returns nil.
func Send(state string) error {
	socketPath := os.Getenv("NOTIFY_SOCKET")
	if socketPath == "" {
		return nil
	}

	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: socketPath,
		Net:  "unixgram",
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	return err
}
