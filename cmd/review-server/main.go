// review-server is a synthetic, single-session review companion. It never creates a real Claude host.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	"github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

const demoID = "00000000-0000-4000-8000-000000000000"

func main() {
	listen := flag.String("listen", "127.0.0.1:18765", "loopback listener")
	public := flag.String("url", "wss://edo.iwhalecloud.com/verfiles/review/ws", "public WSS address")
	data := flag.String("data", "./private-review", "private directory outside web root")
	ttl := flag.Duration("ttl", 7*24*time.Hour, "pairing lifetime")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := serve(ctx, *listen, *public, *data, *ttl); err != nil {
		fmt.Fprintln(os.Stderr, "REVIEW_FAILED: check configuration, private directory and listener")
		os.Exit(1)
	}
}

func validate(listen, public string, ttl time.Duration) error {
	host, _, err := net.SplitHostPort(listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("loopback required")
	}
	u, err := url.Parse(public)
	if err != nil || u.Scheme != "wss" || u.Hostname() == "" || u.Path == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || ttl <= 0 || ttl > 30*24*time.Hour {
		return errors.New("invalid public configuration")
	}
	return nil
}

func exportPair(dir string, p protocol.Pairing) error {
	b, err := p.Marshal()
	if err != nil {
		return err
	}
	png, err := qrcode.Encode(string(b), qrcode.Medium, 512)
	if err != nil {
		return err
	}
	for name, content := range map[string][]byte{"pairing.json": b, "pairing.png": png} {
		tmp := filepath.Join(dir, name+".tmp")
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		if err = f.Chmod(0600); err == nil {
			_, err = f.Write(content)
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if err = os.Rename(tmp, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func serve(ctx context.Context, listen, public, dir string, ttl time.Duration) error {
	if err := validate(listen, public, ttl); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	host, err := claude.NewFakeHost(claude.FakeScenarioInteractive)
	if err != nil {
		return err
	}
	controller := session.NewController(host, session.WithReservedSessionID(demoID), session.WithDisplayMetadata("[DEMO: synthetic data, no commands executed]", time.Now()))
	if err = controller.Start(ctx); err != nil {
		return err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = controller.Stop(stop)
	}()
	manager, err := pairing.NewManager(public, ttl, pairing.WithPersistenceFile(filepath.Join(dir, "credential.json")), pairing.WithPCID("00000000000000000000000000000001"), pairing.WithPCName("REVIEW DEMO - simulated Claude"))
	if err != nil {
		return err
	}
	payload, err := manager.Issue(demoID)
	if err != nil {
		return err
	}
	// Validate encoded files before starting any listener.
	if _, err = json.Marshal(payload); err != nil {
		return err
	}
	if err = exportPair(dir, payload); err != nil {
		return err
	}
	registry := channel.NewRegistry()
	if err = registry.Register(channel.New(demoID, controller)); err != nil {
		return err
	}
	cfg := transport.DefaultConfig()
	cfg.ListenAddress = listen
	cfg.AdvertiseURL = public
	cfg.AllowedOrigins = []string{"capacitor://localhost", "http://localhost", "https://localhost"}
	server, err := transport.NewServer(cfg, manager, registry)
	if err != nil {
		return err
	}
	if err = server.Start(); err != nil {
		return err
	}
	defer func() {
		manager.Invalidate()
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Close(stop)
	}()
	fmt.Println("REVIEW_READY: synthetic demo; pairing files written privately")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			p, rotated, err := manager.RotateIfExpired()
			if err != nil {
				return err
			}
			if rotated {
				if err = exportPair(dir, p); err != nil {
					return err
				}
				fmt.Println("REVIEW_PAIRING_ROTATED")
			}
		}
	}
}
