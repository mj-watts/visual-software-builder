package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"swimlane/backend/internal/studio"
	"syscall"
	"time"
)

func main() {
	if err := serve(); err != nil {
		log.Fatal(err)
	}
}
func serve() error {
	addr := flag.String("addr", "127.0.0.1:8080", "loopback listen address")
	importPath := flag.String("import", "", "optional Python SQLite database to import if destination is absent")
	seed := flag.Bool("seed", true, "seed example tickets only for an empty database")
	flag.Parse()
	if err := studio.LoadEnv(".env"); err != nil {
		return err
	}
	cfg := studio.ConfigFromEnv()
	if *importPath != "" {
		if err := studio.ImportDatabase(*importPath, cfg.Database); err != nil {
			return err
		}
	}
	app, err := studio.New(cfg)
	if err != nil {
		return err
	}
	defer app.Close()
	if *seed {
		if err := app.Seed(); err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go app.Run(ctx)
	server := &http.Server{Addr: *addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Printf("Go backend listening on http://%s (Swagger /docs)", *addr)
	err = server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
