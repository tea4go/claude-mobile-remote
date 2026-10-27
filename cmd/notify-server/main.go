// notify-server accepts notification-only HTTPS proxy requests on edo.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tea4go/claude-mobile-remote/push"
)

func main() {
	address := flag.String("listen", "127.0.0.1:18767", "loopback listen address behind HTTPS proxy")
	flag.Parse()
	host, _, err := net.SplitHostPort(*address)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("listen address must be a loopback IP and port")
	}
	sender, err := push.LoadDefault()
	if err != nil || sender == nil {
		log.Fatal("missing or invalid JPush service configuration")
	}
	deviceFile := os.Getenv("CMR_NOTIFY_DEVICES_FILE")
	if deviceFile == "" {
		deviceFile = "/var/lib/cmr-notify/devices.json"
	}
	devices, err := push.LoadDeviceRegistry(deviceFile)
	if err != nil {
		log.Fatal("missing or invalid notification device store")
	}
	handler, err := push.NewGatewayWithDevices(os.Getenv("CMR_NOTIFY_TOKEN"), sender, devices)
	if err != nil {
		log.Fatal("missing or invalid notification service token")
	}
	server := &http.Server{Addr: *address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Print("notification service starting")
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("notification service listener failed")
	}
	<-stopped
}
