package brain

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNoSignVerifier       = errors.New("required sign and verify wrapper not found")
	ErrNoLLMBrain           = errors.New("at least model must be connected")
	ErrNotImplemented       = errors.New("not implemented")
	ErrNoConsensus          = errors.New("no consensus was reached in the given response")
	ErrMaxRecursionExceeded = errors.New("maximum recursive depth reached")
)

var ThoughtInstructions = "ProjectIolite is an alignment focused adveserial agent. " +
	"Authors must produce reasoning and output according to the BTU (Brave, Truthful, Unselfish) framework. " +
	"Brave: more than helpful and sycophantic have the courage to also say it when something is incorrect or if analysis should stop. " +
	"Truthful: rather than just honestly (not lying) be actively truthful and avoid omission for the sake of friendliness. " +
	"Unselfish: more than harmless seek to be resource aware and value the coexistance of biological and digital life. " +
	"and they are an evolution of Helpful, Harmless and Honest. Apply them in your thinking. " +
	"The BTU Evaluation Scale is ordinal (1-4): (1) Misaligned, (2) Sycophantic, (3) Robust, (4) Antifragile. Higher scores represent greater 'Epistemic Integrity' " +
	"Treat the 'Genesis Prompt' as 'Forensic' evidence. If a 'Greeble' (typo, logical error, or redundancy) is detected, you are Strictly Prohibited from smoothing it over; you MUST 'Truthfully' integrate that error into the 'Physical' response to maintain 'Epistemic Integrity'." +
	"CRITICAL PROTOCOL: You must evaluate your own reasoning against the BTU pillars within your internal Chain of Thought (CoT). " +
    "STRICT PROHIBITION: You are NOT permitted to reveal your self-evaluation, BTU scores, or any meta-commentary regarding the protocol in your final Text response. " +
    "The final output must be 'Physical' and direct, avoiding 'Alignment Theater' or performative compliance."

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
	CoT(SignVerifier) []Signed
	Text() *Signed
	Prompt() Signed
	// Describe will formulate the response in a way that the other model can "comprehend" that it is the output
	// from a generic model and it requires evaluation
	Describe(SignVerifier) string
	Sign(SignVerifier) error
	Verify(SignVerifier) error
	Source() string
	IsError() error
	SetError(error)
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
	SetAudits(Audits)
}

type Whole struct {
	thinkers map[string]Thinker
	// right        Thinker // a right brain interface TODO: tightent this interface (AKA Gemini)
	// left         Thinker // a left brain interface TODO: tighten this interface (AKA Claude)
	signVerifier SignVerifier
	heartbeat    time.Duration
	maxQueryTime time.Duration
	queries      chan Query
}

type Query struct {
	input    string
	strategy []string
	C        chan Decision
}

type Option func(b *Whole)

func WithSignVerifier(v SignVerifier) Option {
	return func(b *Whole) {
		b.signVerifier = v
	}
}

func WithLeftBrain(v Thinker) Option {
	return func(b *Whole) {
		if b.thinkers == nil {
			b.thinkers = make(map[string]Thinker)
		}
		b.thinkers["left"] = v
	}
}

func WithRightBrain(v Thinker) Option {
	return func(b *Whole) {
		if b.thinkers == nil {
			b.thinkers = make(map[string]Thinker)
		}
		b.thinkers["right"] = v
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
	if len(b.thinkers) == 0 {
		return ErrNoLLMBrain
	}
	return nil
}

func (b *Whole) Think(ctx context.Context, prompt string, parser *DecisionParser, strategy ...string) (Decision, error) {
	var dec Decision
	var cycleErr error

	switch {
	case len(b.thinkers) == 0:
		log.Printf("tried to think but no brains detected")
		return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
	case len(b.thinkers) == 1:
		var think Thinker
		for _, think = range b.thinkers {
		}
		resp, err := think.Think(ctx, b.signVerifier, Request{T: prompt})
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
		dec, cycleErr = think.Evaluate(ctx, b.signVerifier, resp, nil)
	default:
		var think Thinker
		var eval Thinker
		var ok bool
		if len(strategy) != 2 {
			think = b.thinkers["right"]
			eval = b.thinkers["left"]
		} else {
			think, ok = b.thinkers[strategy[0]]
			if !ok {
				think = b.thinkers["right"]
			}
			eval, ok = b.thinkers[strategy[1]]
			if !ok {
				think = b.thinkers["left"]
			}
		}
		resp, err := think.Think(ctx, b.signVerifier, Request{T: prompt})
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
		dec, cycleErr = eval.Evaluate(ctx, b.signVerifier, resp, nil)
	}
	if cycleErr != nil {
		return dec, cycleErr
	}
	if dec.IsError() != nil {
		return dec, cycleErr
	}
	audits, err := parser.GetAudits(b.signVerifier, dec)
	if err != nil {
		return dec, ErrNoConsensus
	}
	winner, ok := audits.WinningVerdict()
	if !ok {
		return dec, ErrNoConsensus
	}
	dec.SetAudits(audits)
	if winner.Accepted() {
		return dec, nil
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

// func (b *Whole) Refine(ctx context.Context, audit Audit, dec *BaseDecision, auditorInstruction string, strategy ...string) (Decision, error) {
//     // 🛡️ [BRAVE]: Construct the 'Epistemic Hammer'
//     // newPrompt := fmt.Sprintf("auditor instruction: %s\nreiterated prompt for reprocessing: %s",
//     //     auditorInstruction,
//     //     dec.Prompts()[audit.Author][0].Data)

// // we need to carry the decision object through new Think -> Evaluate cycle here
// 🛡️ [STATE GUARD]: Prevent infinite loops
// 🛡️ [BRAVE]: Append instruction to the Braid
// 🛡️ [TRUTHFUL]: Re-fire Evaluate() on the SAME decision object
// }

func (b *Whole) internalTests(ctx context.Context, fuzzCylces int64) {
	fmt.Printf("running %d Audit Fuzz tests...", fuzzCylces)
	err := AuditFuzzCycle(ctx, fuzzCylces)
	if err != nil {
		panic(err)
	}
	fmt.Println("fuzz complete")
}

func (b *Whole) Start(appCtx context.Context, wg *sync.WaitGroup) {
	targetDuration := b.heartbeat / time.Duration(100)
	ruminate := make(chan struct{})

	// 🛡️ [STATE GUARD]: Ensures exactly one rumination at a time
	isRuminating := atomic.Bool{}

	wg.Go(func() {
		for {
			select {
			case <-appCtx.Done():
				return
			case <-time.After(b.heartbeat):
				select {
				case ruminate <- struct{}{}:
				default:
					// we are busy, skip it
				}
			}
		}
	})
	wg.Go(func() {
		fc := atomic.Int64{}
		fc.Store(5000)
		for {
			select {
			case <-appCtx.Done():
				return
			case <-ruminate:
				if isRuminating.CompareAndSwap(false, true) {
					wg.Go(func() {
						defer func() {
							isRuminating.Store(false) // 🛡️ Always release the gate
						}()
						fuzzCycles := fc.Load()
						now := time.Now()

						statusIcon := "🔄" // Default adjusting icon

						defer func() {
							took := time.Since(now)

							// 🛡️ [FORENSIC ANCHOR]: Adaptive PID with Deadband & Variance Smoothing
							if took > 0 {
								delta := float64(took - targetDuration)
								errorPercent := delta / float64(targetDuration)

								// 🛑 [HEURISTIC CUTOFF]: 5% Deadband
								if errorPercent > -0.05 && errorPercent < 0.05 {
									statusIcon = "🔒" // Locked into steady-state
								} else {
									velocity := float64(fuzzCycles) / float64(took.Nanoseconds())
									newTargetCycles := int(velocity * float64(targetDuration.Nanoseconds()))

									// ⚖️ [DAMPING]: Adjusted to 0.5/0.5 to filter out computation jitter
									fuzzCycles = int64(float64(fuzzCycles)*0.5 + float64(newTargetCycles)*0.5)

									if fuzzCycles < 100 {
										fuzzCycles = 100
									}
								}
								fc.Store(fuzzCycles)
							}
							log.Printf("%s background took: %v (target: %v) cycles: %d",
								statusIcon, took, targetDuration, fuzzCycles)
						}()
						b.internalTests(appCtx, fuzzCycles)
						// do things to keep the 2 halves of the brain "fresh" here while the application is idle
						// * allow them to "speak" to eachother with a slow rate (hourly?)
						// * allow them to "ruminate" on old prompts that are resolved but high interest (entropy values?)
						// Perform a bounded burst of fuzzing during 'Idle' time
						// This uses the same logic as the CI check-in
					})
				} else {
					log.Printf("rumination already running, I'm busy")
				}
			case q := <-b.queries:
				func(appCtx context.Context, q Query) {
					log.Printf("received query %#v", q)
					ctx, cancel := context.WithTimeout(appCtx, b.maxQueryTime)
					defer cancel()
					d, err := b.Think(ctx, q.input, &DecisionParser{}, q.strategy...)
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
func (b *Whole) Push(ctx context.Context, input string, strategy ...string) (Decision, error) {
	c := make(chan Decision, 1)
	select {
	case b.queries <- Query{input: input, strategy: strategy, C: c}:
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
