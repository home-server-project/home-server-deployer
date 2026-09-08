package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	deployerweb "github.com/home-server-project/home-server-deployer/internal/web"
)

func main() {
	socket := envOr("DEPLOYER_AGENT_SOCKET", "/run/home-server-deployer/agent.sock")
	listen := envOr("DEPLOYER_WEB_LISTEN", ":8080")
	ui, err := deployerweb.NewServer(deployerweb.NewAgentClient(socket))
	if err != nil {
		log.Fatal(err)
	}
	s := &http.Server{Addr: listen, Handler: ui.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Home Server Deployer web listening on %s", listen)
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
