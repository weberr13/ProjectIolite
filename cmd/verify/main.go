package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weberr13/ProjectIolite/brain"
	"github.com/weberr13/ProjectIolite/jwtwrapper"
)

func main() {
	appContext, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// TODO: the SignedVerifier requires its own keys, not really used here
	// so the constructor could be modifed later.  Not a big deal right now
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	sv := jwtwrapper.New(pub, priv)

	var data brain.SignedContainer

	supportedFormats := []brain.SignedContainer{
		&brain.SignedHydration{},
		&brain.BaseDecision{},
	}

	r, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no input found")
		os.Exit(1)
	}
	sanitized := brain.SanitizeJSON(r)
	for _, f := range supportedFormats {
		data = f
		err = json.NewDecoder(bytes.NewBuffer(sanitized)).Decode(&data)
		if err != nil {
			continue
		}
		if len(data.Blocks()) != 0 && data.Blocks()[0].Data != "" {
			break
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", string(r))
		fmt.Fprintf(os.Stderr, "[BTU_ERROR]: Failed to parse braid JSON: %v\n", err)
		os.Exit(1)
	}

	log.Printf("found block of type %T", data)
	err = brain.VerifyBraid(appContext, sv, data)
	if err != nil {
		log.Printf("VERIFICATION FAILED: %s\n", err)
		os.Exit(1)
	}
	log.Printf("braid verified")
}
