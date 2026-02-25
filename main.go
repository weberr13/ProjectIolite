package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/weberr13/ProjectIolite/brain"
	"github.com/weberr13/ProjectIolite/jwtwrapper"
)

func main() {
	appContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("got signal %s", sig)
		go func() {
			time.Sleep(10 * time.Second)
			os.Exit(1)
		}()
		cancel()
	}()


	// TODO: Persist and reload these at start if specified in flags
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        panic(err)
    }

	sv := jwtwrapper.New(pub, priv)
	backend, err := brain.NewWhole(brain.WithSignVerifier(sv),brain.WithRightBrain("not a brain"))
	if err != nil {
		panic(err)
	}
	backend.Start(appContext, wg)
	wg.Wait()
}
