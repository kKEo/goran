// Command goran-agent registers with a Goran server and executes its tasks.
//
//	goran-agent register --server URL --token REG_TOKEN --name NAME [--labels a,b]
//	goran-agent run [--config agent.json] [--workdir ./work] [--poll 2s]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kkEo/g-mk8s/agent/client"
	"github.com/kkEo/g-mk8s/agent/config"
	"github.com/kkEo/g-mk8s/agent/worker"
	"github.com/kkEo/g-mk8s/wire"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "register":
		err = register(os.Args[2:])
	case "run":
		err = run(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: goran-agent <command> [flags]

commands:
  register   exchange a one-time registration token for an agent key and save it
  run        poll the server and execute tasks

environment: GORAN_SERVER, GORAN_AGENT_KEY, GORAN_AGENT_CONFIG (default agent.json),
  GORAN_AGENT_NAME, GORAN_AGENT_LABELS, GORAN_WORKDIR, GORAN_POLL_SECONDS,
  GORAN_REGISTRATION_TOKEN`)
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func register(args []string) error {
	host, _ := os.Hostname()
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	server := fs.String("server", envOr("GORAN_SERVER", ""), "server URL, e.g. http://localhost:8080")
	token := fs.String("token", envOr("GORAN_REGISTRATION_TOKEN", ""), "one-time registration token from the server")
	name := fs.String("name", envOr("GORAN_AGENT_NAME", host), "agent name")
	labels := fs.String("labels", envOr("GORAN_AGENT_LABELS", ""), "comma separated labels tasks can require")
	cfgPath := fs.String("config", config.DefaultPath(), "where to save the agent key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *server == "" || *token == "" {
		return errors.New("--server and --token are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := client.New(*server, "").Register(ctx, wire.RegisterRequest{
		Token: *token, Name: *name, Labels: config.SplitLabels(*labels),
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	cfg := &config.Config{Server: *server, Key: resp.Key, Name: resp.Name, Labels: config.SplitLabels(*labels)}
	if err := cfg.Save(*cfgPath); err != nil {
		return err
	}
	fmt.Printf("registered agent %q (id %d) in workspace %d\nkey saved to %s\n", resp.Name, resp.AgentID, resp.WorkspaceID, *cfgPath)
	return nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "agent config written by register")
	server := fs.String("server", "", "server URL (overrides config and GORAN_SERVER)")
	key := fs.String("key", "", "agent key (overrides config and GORAN_AGENT_KEY)")
	workdir := fs.String("workdir", "", "directory for task checkouts and plans (default ./work)")
	poll := fs.Duration("poll", 0, "poll interval (default 2s)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cfg.ApplyEnv()
	if *server != "" {
		cfg.Server = *server
	}
	if *key != "" {
		cfg.Key = *key
	}
	if *workdir != "" {
		cfg.WorkDir = *workdir
	}
	if *poll > 0 {
		cfg.PollSeconds = int(poll.Seconds())
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	wd := cfg.WorkDirOrDefault()
	if err := os.MkdirAll(wd, 0o755); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	w := &worker.Worker{
		Client:  client.New(cfg.Server, cfg.Key),
		WorkDir: wd,
		Poll:    cfg.PollInterval(),
		Logf:    log.Printf,
	}
	log.Printf("goran-agent %q starting", cfg.Name)
	err = w.Run(ctx)
	log.Println("goran-agent stopped")
	return err
}
