// Command goran-server runs the Goran control plane.
//
//	goran-server serve      start the API and console (default)
//	goran-server bootstrap  create the first admin user, workspace and token
//	goran-server keygen     print a new GORAN_MASTER_KEY
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kkEo/g-mk8s/webapp/api"
	"github.com/kkEo/g-mk8s/webapp/db"
	"github.com/kkEo/g-mk8s/webapp/util"
)

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "bootstrap":
		err = bootstrap(args)
	case "keygen":
		err = keygen()
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: goran-server <command> [flags]

commands:
  serve       start the API server and console (default)
  bootstrap   create the first admin user, a workspace and print a token
  keygen      print a fresh master key for GORAN_MASTER_KEY

environment (flags override): GORAN_ADDR, GORAN_DB, GORAN_MASTER_KEY,
  GORAN_LEASE_SECONDS, GORAN_MAX_ATTEMPTS, GORAN_TASK_TIMEOUT_SECONDS, GORAN_LOG_SQL`)
}

func envOr(name, def string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return def
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(envOr(name, "")); err == nil {
		return v
	}
	return def
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", envOr("GORAN_ADDR", ":8080"), "listen address")
	dbPath := fs.String("db", envOr("GORAN_DB", "local.sqlite"), "SQLite database file")
	masterKey := fs.String("master-key", envOr("GORAN_MASTER_KEY", ""), "hex encoded 32-byte key for secrets at rest")
	lease := fs.Int("lease-seconds", envInt("GORAN_LEASE_SECONDS", 60), "seconds an agent may stay silent before its task is requeued")
	maxAttempts := fs.Int("max-attempts", envInt("GORAN_MAX_ATTEMPTS", 3), "how many times a task is handed out before it errors")
	timeout := fs.Int("task-timeout-seconds", envInt("GORAN_TASK_TIMEOUT_SECONDS", 3600), "default task timeout")
	logSQL := fs.Bool("log-sql", envOr("GORAN_LOG_SQL", "") == "1", "log every SQL statement")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *masterKey == "" {
		return errors.New("GORAN_MASTER_KEY is required so secrets can be encrypted at rest; run `goran-server keygen` to create one")
	}
	key, err := util.ParseKey(*masterKey)
	if err != nil {
		return err
	}
	box, err := util.NewBox(key)
	if err != nil {
		return err
	}
	database, err := db.Open(db.Options{Path: *dbPath, LogSQL: *logSQL})
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	srv := api.New(api.Config{
		DB:             database,
		Box:            box,
		Lease:          time.Duration(*lease) * time.Second,
		MaxAttempts:    *maxAttempts,
		DefaultTimeout: time.Duration(*timeout) * time.Second,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv.StartReaper(ctx, time.Duration(*lease)*time.Second/2)

	httpSrv := &http.Server{Addr: *addr, Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()
	log.Printf("goran-server listening on %s (db %s, lease %ds)", *addr, *dbPath, *lease)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Println("goran-server stopped")
	return nil
}

func bootstrap(args []string) error {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	dbPath := fs.String("db", envOr("GORAN_DB", "local.sqlite"), "SQLite database file")
	user := fs.String("user", "admin", "admin user name")
	email := fs.String("email", "", "admin email")
	workspace := fs.String("workspace", "default", "first workspace name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	database, err := db.Open(db.Options{Path: *dbPath})
	if err != nil {
		return err
	}
	res, err := api.Bootstrap(database, *user, *email, *workspace)
	if err != nil {
		return err
	}
	fmt.Printf("user:      %s (admin)\nworkspace: %s\ntoken:     %s\n\nexport GORAN_TOKEN=%s\n",
		res.User.Name, res.Workspace.Name, res.Token, res.Token)
	return nil
}

func keygen() error {
	k, err := util.NewKey()
	if err != nil {
		return err
	}
	fmt.Printf("export GORAN_MASTER_KEY=%s\n", util.EncodeKey(k))
	return nil
}
