//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"goft/postgres"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"golang.org/x/crypto/bcrypt"

	// import .env
	_ "github.com/joho/godotenv/autoload"
)

func seedRooms(DB *pgxpool.Pool) error {
	query := `
	INSERT INTO rooms(name, description)
	VALUES($1, $2)
	`

	data := []struct {
		name        string
		description string
	}{
		{"Tech Talk 💻", "A place to discuss the latest in technology and gadgets."},
		{"Book Club 📚", "Share your favorite reads and discover new books."},
		{"Travel Buddies ✈️", "Connect with fellow travelers and share tips."},
		{"Fitness Fanatics 💪", "Discuss workouts, nutrition, and wellness."},
		{"Gaming Zone 🎮", "Join fellow gamers to chat about your favorite games."},
		{"Movie Buffs 🎬", "Talk about the latest films and classic favorites."},
		{"Music Lovers 🎶", "Share playlists and discover new artists."},
		{"Cooking Corner 🍳", "Exchange recipes and cooking tips."},
		{"Art & Design 🎨", "Discuss art techniques and showcase your work."},
		{"Pet Lovers 🐾", "Share stories and tips about your furry friends."},
	}

	for _, d := range data {
		_, err := DB.Exec(context.Background(), query, d.name, d.description)
		if err != nil {
			return err
		}
	}

	return nil
}

func seedUsers(DB *pgxpool.Pool) error {
	query := `
	INSERT INTO users(name, hashed_password)
	VALUES($1, $2)
	`

	name := "test"
	password := "123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password, %v", err)
	}

	_, err = DB.Exec(context.Background(), query, name, hashedPassword)
	if err != nil {
		return err
	}

	return nil
}

// seeds the database with initial data
func Seed() error {
	pg, err := postgres.New()
	if err != nil {
		return err
	}
	defer pg.Close()

	err = seedUsers(pg.DB)
	if err != nil {
		return err
	}

	err = seedRooms(pg.DB)
	if err != nil {
		return err
	}

	return nil
}

// migrate the database to newset version
func Migrate() error {
	return sh.Run("go", "tool", "-modfile", "tools.mod", "goose", "up")
}

// installs project dependencies
func Install() error {
	err := sh.Run("npm", "install")
	if err != nil {
		return err
	}

	err = sh.Run("go", "mod", "tidy")
	if err != nil {
		return err
	}

	err = sh.Run("go", "mod", "tidy", "-modfile", "tools.mod")
	if err != nil {
		return err
	}

	return nil
}

// builds project
func Build() error {
	mg.Deps(Install)
	fmt.Println("Building to ./bin/server ...")
	return sh.Run("go", "build", "-o", "bin/server", ".")
}

// basically install + migrate + seed
func Init() error {
	fmt.Println("installing dependencies...")

	err := Install()
	if err != nil {
		return err
	}

	fmt.Println("database migrations...")
	err = Migrate()
	if err != nil {
		return err
	}

	fmt.Println("Seeding database...")
	err = Seed()
	if err != nil {
		return err
	}

	return nil
}

var commands = []string{
	"npx tailwindcss -i ./views/css/tailwindcss.css -o ./static/css/style.css -w always",
	"go tool -modfile tools.mod air .",
	"go tool -modfile tools.mod templ generate --watch",
}

const KILL_TIMEOUT = 10

// starts the development server
func Run() {
	quit := make(chan struct{})

	var wg sync.WaitGroup

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	cmds := make([]*exec.Cmd, len(commands))

	for i, c := range commands {
		go func() {
			defer wg.Done()

			cmd := exec.Command("sh", "-c", c)
			cmds[i] = cmd

			stdout, err := cmd.StdoutPipe()
			// combined output
			cmd.Stderr = cmd.Stdout
			if err != nil {
				fmt.Println(err)
				close(quit)
				return
			}

			err = cmd.Start()
			if err != nil {
				fmt.Println(err)
				close(quit)
				return
			}

			go func() {
				err = cmd.Wait()
				if err != nil {
					fmt.Println(err)
					close(quit)
					return
				}
			}()

			// read output chunk by chunk
			for {
				tmp := make([]byte, 1024)
				_, err := stdout.Read(tmp)
				if err != nil {
					break
				}
				fmt.Print(string(tmp))
			}
		}()

		wg.Add(1)
	}

	go func() {
		wg.Wait()
		close(quit)
	}()

	go func() {
		// wait for user interrupt
		<-sig

		for _, c := range cmds {
			done := make(chan struct{})
			go func() {
				// send signal and wait for exit
				err := c.Process.Signal(syscall.SIGTERM)
				if err != nil {
					log.Printf("%q: failed to send SIGTERM signal: %s\n", c, err)
					return
				}

				err = c.Wait()
				if err != nil {
					log.Printf("%q: failed to wait for process: %s\n", c, err)
				}

				close(done)
			}()

			select {
			case <-done:
				continue
			case <-time.After(KILL_TIMEOUT * time.Second):
				c.Process.Kill()
			}
		}
		close(quit)
	}()

	<-quit
}
