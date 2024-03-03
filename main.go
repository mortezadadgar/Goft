package main

import (
	"context"
	"goft/chat"
	"goft/postgres"
	"goft/server"
	sessionstore "goft/sessionStore"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	// .env
	_ "github.com/joho/godotenv/autoload"
)

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pg, err := postgres.New()
	if err != nil {
		return err
	}

	session := sessionstore.New(pg)
	room := chat.New()

	server := server.New(pg, room, session)
	errc := server.Start()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer pg.DB.Close()
		defer wg.Done()

		select {
		case <-ctx.Done():
			if err := server.Close(); err != nil {
				log.Printf("failed to shutting down server: %s\n", err)
				os.Exit(1)
			}
		case err := <-errc:
			log.Printf("failed to start server: %s\n", err)
		}
	}()

	wg.Wait()
	return nil
}

func main() {
	log.SetFlags(log.Lshortfile | log.Ltime)
	if err := run(); err != nil {
		log.Printf("%s\n", err)
	}
}
