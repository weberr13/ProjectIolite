package brain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNoSignVerifier = errors.New("required sign and verify wrapper not found")
	ErrNoLLMBrain = errors.New("at least model must be connected")
)

type SignVerifier interface {
	Sign(data string) (string, error)
	Verify(data, signature string) error
}

type Whole struct {
	right any // a right brain interface TODO: tightent this interface (AKA Gemini)
	left any // a left brain interface TODO: tighten this interface (AKA Claude)
	signVerifier SignVerifier
	heartbeat time.Duration
}

type Option func(b *Whole)

func WithSignVerifier(v SignVerifier) Option {
	return func(b *Whole) {
		b.signVerifier = v
	}
}

func WithLeftBrain(v any) Option {
	return func(b *Whole) {
		b.left = v
	}
}

func WithRightBrain(v any) Option {
	return func(b *Whole) {
		b.right = v
	}
}

func WithHeartbeatTime(d time.Duration) Option {
	return func(b *Whole) {
		b.heartbeat = d
	}
}

func NewWhole(opt ...Option) (*Whole, error) {
	b := &Whole{
		heartbeat: 5*time.Second,
	}

	for _, o := range opt {
		o(b)
	}

	return b, b.Ready()
}

// Ready checks that all the internal state is loaded and set correctly
func (b *Whole) Ready() error {
	if b.signVerifier == nil {
		return ErrNoSignVerifier
	}
	if b.left == nil && b.right == nil {
		return ErrNoLLMBrain
	}
	return nil 
}

func (b *Whole) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <- ctx.Done(): 
				return
			case <- time.After(b.heartbeat):
				fmt.Println(".")
			}
		}
	}()
}