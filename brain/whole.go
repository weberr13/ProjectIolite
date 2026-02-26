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
	ErrNoLLMBrain     = errors.New("at least model must be connected")
	ErrNotImplemented = errors.New("not implemented")

	DecodePy = `import base64
from cryptography.hazmat.primitives.asymmetric import ed25519

def verify_iolite_signature(pub_key_base64, data, signature_base64):
    # Decode the key and signature from the "Signed" struct
    public_key_bytes = base64.b64decode(pub_key_base64)
    signature_bytes = base64.b64decode(signature_base64)
    
    # Load the Ed25519 public key
    public_key = ed25519.Ed25519PublicKey.from_public_bytes(public_key_bytes)
    
    try:
        # Verify the data (the 'Handshake Request' or CoT string)
        public_key.verify(signature_bytes, data.encode('utf-8'))
        return True
    except Exception:
        return False

# Example usage from a model's 'Evaluation' pass
# pub_key = "..." from Go ExportPublicKey()
# result = verify_iolite_signature(pub_key, "the thought string", "the_signature")`
)

type SignVerifier interface {
	Sign(data string) (string, error)
	Verify(data, signature string) error
	ExportPublicKey() string
	Alg() string
}

type Thinker interface {
	// Think generates the initial response
	Think(ctx context.Context, sv SignVerifier, input Request) (Response, error)
	// Evaluate audits another brain's output
	Evaluate(ctx context.Context, sv SignVerifier, peerOutput Response, prev Decision) (Decision, error)
}

type Request struct {
	t string
}

func (r *Request) Text() string {
	return r.t
}

type Signed struct {
	Data      string
	Signature string
}

type Response interface {
	CoT() []Signed
	Text() *Signed
	Prompt() Signed
	// Describe will formulate the response in a way that the other model can "comprehend" that it is the output
	// from a generic model and it requires evaluation
	Describe(SignVerifier) string
	Sign(SignVerifier) error
	Verify(SignVerifier) error
}

type Decision interface {
	Cots() map[string][][]Signed  // map of model -> collections of CoTs indexed by rebuttal turn order
	Prompts() map[string][]Signed // the sequence of prompts given to each model
	Texts() map[string][]Signed   // map of model -> turn responses
	Sign(SignVerifier) error
	Verify(SignVerifier) error
	Add(cot []Signed, text Signed, sv SignVerifier) error
}

type Whole struct {
	right        Thinker // a right brain interface TODO: tightent this interface (AKA Gemini)
	left         Thinker // a left brain interface TODO: tighten this interface (AKA Claude)
	signVerifier SignVerifier
	heartbeat    time.Duration
	maxQueryTime time.Duration
	queries      chan Query
}

type Query struct {
	input string
	C     chan Decision
}

type Option func(b *Whole)

func WithSignVerifier(v SignVerifier) Option {
	return func(b *Whole) {
		b.signVerifier = v
	}
}

func WithLeftBrain(v Thinker) Option {
	return func(b *Whole) {
		b.left = v
	}
}

func WithRightBrain(v Thinker) Option {
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
		heartbeat:    5 * time.Second,
		maxQueryTime: 60 * time.Second,
		queries:      make(chan Query, 10), // optionally tune this depth later
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

func (b *Whole) Think(ctx context.Context, prompt string) (Decision, error) {
	switch {
	case b.left == nil && b.right == nil:
		return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
	case b.left == nil:
		resp, err := b.right.Think(ctx, b.signVerifier, Request{t: prompt})
		if err != nil {
			return &ErrorDecision{E: err}, err
		}
		return b.right.Evaluate(ctx, b.signVerifier, resp, nil)
	case b.right == nil:
		resp, err := b.left.Think(ctx, b.signVerifier, Request{t: prompt})
		if err != nil {
			return &ErrorDecision{E: err}, err
		}
		return b.left.Evaluate(ctx, b.signVerifier, resp, nil)
	}
	// TODO: when we have 2 halves we can implement the debate logic

	// 	Will choose either right or left brain based on a criteria (random, text length, etc, TBD) and send the prompt
	// to the current context there. Once a response and CoT is generated from the chosen half the other half will
	// then be given an "evaluation period" where the prompt, the response and the other side's signed CoT will be
	// presented along side instructions to either

	// 1) reject the logic of the other side entirely, produce a rebuttal
	// 2) accept the logic without comment -> output is returned from Think
	// 3) augment the logic while agreeing but add additional context or ideas

	// in case 1 the rebuttal will be given to the first half and that half will take on the "reviewer" role directly
	// ignoring its original train of thought and try to reach a consensus.

	// in case 3 there will be a program counter for how many "back and forth" exchanges are permitted before the
	// last result "wins" (a small number)
	return &ErrorDecision{
		E: ErrNotImplemented,
	}, ErrNotImplemented
}

func (b *Whole) Start(appCtx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-appCtx.Done():
				return
			case <-time.After(b.heartbeat):
				fmt.Println(".")
				// do things to keep the 2 halves of the brain "fresh" here while the application is idle
				// * allow them to "speak" to eachother with a slow rate (hourly?)
				// * allow them to "ruminate" on old prompts that are resolved but high interest (entropy values?)
			case q := <-b.queries:
				func(appCtx context.Context, q Query) {
					ctx, cancel := context.WithTimeout(appCtx, b.maxQueryTime)
					defer cancel()
					d, err := b.Think(ctx, q.input)
					if err != nil {
						d = &ErrorDecision{
							E: err,
						}
					}
					select {
					case q.C <- d:
					case <-ctx.Done():
					}
				}(appCtx, q)
			}
		}
	}()
}
