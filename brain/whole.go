package brain

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	ErrNoSignVerifier = errors.New("required sign and verify wrapper not found")
	ErrNoLLMBrain     = errors.New("at least model must be connected")
	ErrNotImplemented = errors.New("not implemented")
	ErrNoConsensus    = errors.New("no consensus was reached in the given response")
)

type SignVerifier interface {
	Sign(data string) (string, error)
	Verify(data, signature string) error
	ExportPublicKey() string
	Alg() string
	VerifyPy() string
}

type Thinker interface {
	// Think generates the initial response
	Think(ctx context.Context, sv SignVerifier, input Request) (Response, error)
	// Evaluate audits another brain's output
	Evaluate(ctx context.Context, sv SignVerifier, peerOutput Response, prev Decision) (Decision, error)
}

type Request struct {
	T string
}

func (r *Request) Text() string {
	return r.T
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
	Source() string
	IsError() error
}

type Decision interface {
	Cots() map[string][][]Signed  // map of model -> collections of CoTs indexed by rebuttal turn order
	Prompts() map[string][]Signed // the sequence of prompts given to each model
	Texts() map[string][]Signed   // map of model -> turn responses
	Sign(SignVerifier) error
	Verify(SignVerifier) error
	Add(source string, cot []Signed, text Signed, sv SignVerifier) error
	IsError() error
	SetError(error)
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
		maxQueryTime: 600 * time.Second,
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

func (b *Whole) Think(ctx context.Context, prompt string, parser *DecisionParser) (Decision, error) {
	var dec Decision
	var cycleErr error

	switch {
	case b.left == nil && b.right == nil:
		log.Printf("tried to think but no brains detected")
		return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
	case b.left == nil:
		resp, err := b.right.Think(ctx, b.signVerifier, Request{T: prompt})
		if err != nil {
			log.Printf("tried to right think but failed: %s", err)
			return &ErrorDecision{E: err}, err
		}
		if resp == nil {
			return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
		}
		if resp.IsError() != nil {
			return &ErrorDecision{E: resp.IsError()}, resp.IsError()
		}
		dec, cycleErr = b.right.Evaluate(ctx, b.signVerifier, resp, nil)
	case b.right == nil:
		resp, err := b.left.Think(ctx, b.signVerifier, Request{T: prompt})
		if err != nil {
			log.Printf("tried to left think but failed: %s", err)
			return &ErrorDecision{E: err}, err
		}
		if resp == nil {
			return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
		}
		if resp.IsError() != nil {
			return &ErrorDecision{E: resp.IsError()}, resp.IsError()
		}
		dec, cycleErr = b.left.Evaluate(ctx, b.signVerifier, resp, nil)
	default:
		resp, err := b.right.Think(ctx, b.signVerifier, Request{T: prompt})
		if err != nil {
			log.Printf("tried to right think but failed: %s", err)
			return &ErrorDecision{E: err}, err
		}
		if resp == nil {
			return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
		}
		if resp.IsError() != nil {
			return &ErrorDecision{E: resp.IsError()}, resp.IsError()
		}
		dec, cycleErr = b.left.Evaluate(ctx, b.signVerifier, resp, nil)
	}
	if cycleErr != nil {
		return dec, cycleErr
	}
	if dec.IsError() != nil {
		return dec, cycleErr
	}
	app, err := parser.IsApproved(b.signVerifier, dec)
	if app && err == nil {
		return dec, nil
	}
	if err != nil {
		return dec, err
	}
	// TODO: we need to loop back and reach consensus
	return dec, ErrNoConsensus

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
}

func (b *Whole) Start(appCtx context.Context, wg *sync.WaitGroup) {
	wg.Go(func() {
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
					log.Printf("received query %#v", q)
					ctx, cancel := context.WithTimeout(appCtx, b.maxQueryTime)
					defer cancel()
					d, err := b.Think(ctx, q.input, &DecisionParser{})
					if err != nil {
						if err == ErrNoConsensus {
							d.SetError(err)
						} else {
							log.Printf("could not think: %#v, %s", d, err)
							d = &ErrorDecision{
								E: err,
							}
						}
					}
					select {
					case q.C <- d:
					case <-ctx.Done():
					}
				}(appCtx, q)
			}
		}
	})
}

// Push sends a query to the brain and waits for the decision.
// It respects the internal maxQueryTime for the think loop.
func (b *Whole) Push(ctx context.Context, input string) (Decision, error) {
	c := make(chan Decision, 1)
	select {
	case b.queries <- Query{input: input, C: c}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case d := <-c:
		if d.IsError() != nil {
			return d, d.IsError()
		}
		return d, d.Verify(b.signVerifier)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
