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
	"path/filepath"
	"syscall"
	"time"

	"github.com/home-server-project/home-server-deployer/internal/api"
	"github.com/home-server-project/home-server-deployer/internal/catalog"
	"github.com/home-server-project/home-server-deployer/internal/config"
	"github.com/home-server-project/home-server-deployer/internal/docsgen"
	"github.com/home-server-project/home-server-deployer/internal/engine"
	"github.com/home-server-project/home-server-deployer/internal/podman"
	"github.com/home-server-project/home-server-deployer/internal/security/selinux"
	"github.com/home-server-project/home-server-deployer/internal/state"
	hostsystemd "github.com/home-server-project/home-server-deployer/internal/systemd"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", envOr("DEPLOYER_CONFIG", config.DefaultPath), "configuration file")
	flag.Parse()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}
	st := state.New(cfg.StateRoot)
	if err := st.Init(); err != nil {
		log.Fatal(err)
	}
	cat, err := catalog.Load(cfg.CatalogRoot)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sd, err := hostsystemd.NewDirect(ctx)
	if err != nil {
		log.Fatalf("connect directly to systemd: %v", err)
	}
	defer sd.Close()
	eng := engine.New(podman.New(cfg.PodmanSocket), sd, cat, st, docsgen.New(cfg.DocumentationRoot), selinux.New(), cfg)
	if err := os.MkdirAll(filepath.Dir(cfg.ListenSocket), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.Remove(cfg.ListenSocket); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", cfg.ListenSocket)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	if err := os.Chmod(cfg.ListenSocket, 0666); err != nil {
		log.Fatal(err)
	}
	// The shared runtime volume is mounted only into Agent and Web. The socket
	// is world-accessible inside that private volume so the Agent can drop all
	// Linux capabilities and the Web can remain an unprivileged fixed UID.
	srv := &http.Server{Handler: api.New(eng).Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("agent server: %v", err)
		}
	}()
	log.Printf("Home Server Deployer agent listening on unix:%s", cfg.ListenSocket)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_ = srv.Shutdown(shutdown)
}
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
