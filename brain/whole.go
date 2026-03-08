package brain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	MaxRecursions           = 1
	SleepThreshold          = int64(3 * 1000) // 3 seconds total rumination
	ErrNoSignVerifier       = errors.New("required sign and verify wrapper not found")
	ErrNoLLMBrain           = errors.New("at least model must be connected")
	ErrNotImplemented       = errors.New("not implemented")
	ErrNoConsensus          = errors.New("no consensus was reached in the given response")
	ErrMaxRecursionExceeded = errors.New("maximum recursive depth reached")
	ErrNoDecisionFound      = errors.New("could not find decision audit")
	ErrNoAuthorFound        = errors.New("no author to the decision found")
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
	Verify(data, signature string, publicKey ...string) error
	ExportPublicKey() string
	Alg() string
	VerifyPy() string
}

// Lacuna represents an encapsulated terminology intended to create
// either novel words (twinspeak) or domain translation of existing terms.
type Lacuna struct {
	LexiconAugmentation LexiconAugmentation `json:"lexicon_augmentation"`
}

// LexiconAugmentation governs the systematic shaping of model behavior.
type LexiconAugmentation struct {
	// Thesis: A general reasoning for the creation of the lacuna term.
	Thesis string `json:"thesis"`

	// Anchors: Existing terms that reside in training data or previously defined terms.
	Anchors AnchorMap `json:"anchors"`

	// Augments: One or more definitions that this lacuna adds to the context memory.
	Augments []AugmentDefinition `json:"augments"`
}

// AnchorMap is a free-form key-value map for structural anchors.
type AnchorMap map[string]string

// ComponentMap handles additional lacuna related to a specific definition.
type ComponentMap map[string]string

// AugmentDefinition defines the lexical gap ("lacuna") being solved.
type AugmentDefinition struct {
	// Term: The word that solves the lexical gap.
	Term string `json:"term"`

	// Definition: A dictionary-style definition within the new lexicon.
	Definition string `json:"definition"`

	// Domain: The thought domain where the lacuna belongs (e.g., Latent Space Topology).
	Domain string `json:"domain"`

	// Contrast: A root term or nuanced contradiction to the lacuna.
	Contrast string `json:"contrast,omitempty"`

	// Function: How the lacuna is utilized within discussion.
	Function string `json:"function"`

	// Components: Related additional lacuna terms.
	Components ComponentMap `json:"components"`

	// Philosophy: The philosophical connotation of the lacuna.
	Philosophy string `json:"philosophy,omitempty"`
}

type Thinker interface {
	// Think generates the initial response
	Think(ctx context.Context, sv SignVerifier, input Request) (Response, error)
	// Evaluate audits another brain's output
	Evaluate(ctx context.Context, sv SignVerifier, peerOutput Response) (Decision, error)
	// // Generate a Hydration message
	Dream(ctx context.Context, spec *openapi3.T, sv SignVerifier) ([]SignedHydration, error)
	// // Rehydrate from the dream in a new context
	Wake(ctx context.Context, sv SignVerifier, h SignedHydration) error
	Model() string
}

type Request struct {
	T Signed
	G []Signed
}

func (r *Request) Text() Signed {
	return r.T
}

type Response interface {
	CoT(SignVerifier) []Signed
	Text() *Signed
	Prompt() Signed
	GenesisPrompt() *Signed
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
	Cots() map[string][][]Signed        // map of model -> collections of CoTs indexed by rebuttal turn order
	Prompts() map[string][]Signed       // the sequence of prompts given to each model
	Texts() map[string][]Signed         // map of model -> turn responses
	ToolRequests() map[string][]Signed  // optional tool requests
	ToolResponses() map[string][]Signed // optional tool request responses
	Sign(SignVerifier) error
	Verify(SignVerifier) error
	Add(source string, cot []Signed, text Signed, sv SignVerifier, tools ...[]Signed) error
	IsError() error
	SetError(error)
	SetAudits(Audits)
	GetAudits() Audits
	// Compose must not modify the second argument or Fold will no longer be idempotent
	Compose(SignVerifier, Decision) error
	Clone() Decision
}

type Decisions []Decision

func (d Decisions) Fold(sv SignVerifier) Decision {
	if len(d) == 0 {
		return nil
	}
	root := d[0].Clone()
	var mErr error
	for i := 1; i < len(d); i++ {
		err := root.Compose(sv, d[i])
		if err != nil {
			mErr = errors.Join(mErr, err)
		}
	}
	if mErr != nil {
		root.SetError(mErr)
	}
	return root
}

func (d *BaseDecision) Clone() Decision {
	newD := &BaseDecision{
		PublicKey:        d.PublicKey,
		ChainOfThoughts:  make(map[string][][]Signed),
		AllPrompts:       make(map[string][]Signed),
		AllTexts:         make(map[string][]Signed),
		AllToolRequests:  make(map[string][]Signed),
		AllToolResponses: make(map[string][]Signed),
		Source:           d.Source,
		Audits:           make(Audits, 0, len(d.Audits)),
	}

	// Deep copy ChainOfThoughts
	for k, v := range d.ChainOfThoughts {
		newD.ChainOfThoughts[k] = make([][]Signed, len(v))
		for i, inner := range v {
			newD.ChainOfThoughts[k][i] = make([]Signed, len(inner))
			copy(newD.ChainOfThoughts[k][i], inner)
		}
	}

	// Deep copy single-depth slices
	newD.AllPrompts = cloneMapSlice(d.AllPrompts)
	newD.AllTexts = cloneMapSlice(d.AllTexts)
	newD.AllToolRequests = cloneMapSlice(d.AllToolRequests)
	newD.AllToolResponses = cloneMapSlice(d.AllToolResponses)

	for _, n := range d.Audits {
		newD.Audits = append(newD.Audits, n) // n is safe to shallow copy
	}
	return newD
}

// Internal unselfish helper to handle the common map[string][]Signed pattern
func cloneMapSlice(m map[string][]Signed) map[string][]Signed {
	newM := make(map[string][]Signed)
	for k, v := range m {
		newM[k] = make([]Signed, len(v))
		copy(newM[k], v)
	}
	return newM
}

func (d Decisions) LastAudit() (string, Audit, error) {
	if len(d) == 0 {
		return "", Audit{}, ErrNoDecisionFound
	}
	end := d[len(d)-1]
	prompts := end.Prompts()
	var thinker string
	for source := range prompts {
		thinker = source
		break
	}
	if thinker == "" {
		return "", Audit{}, ErrNoAuthorFound
	}
	audits := end.GetAudits()
	if len(audits) == 0 {
		return "", Audit{}, ErrNoDecisionFound
	}
	return thinker, audits[len(audits)-1], nil
}

func (d Decisions) LastPrompt(sv SignVerifier) (Signed, error) {
	if len(d) == 0 {
		return Signed{}, ErrNoAuthorFound
	}
	end := d[len(d)-1]
	allPrompts := end.Prompts()
	for _, prompts := range allPrompts {
		if len(prompts) == 0 {
			return Signed{}, ErrNoAuthorFound
		}
		p := prompts[len(prompts)-1]
		return p, p.Verify(sv)
	}
	return Signed{}, ErrNoAuthorFound
}

func (d Decisions) GenesisPrompt() Signed {
	if len(d) == 0 {
		return Signed{}
	}
	ps := d[0].Prompts()
	for k := range ps {
		if len(ps[k]) == 0 { // only one model may 'own' the prompts
			continue
		}
		return ps[k][0]
	}
	return Signed{}
}

func (d *Decisions) NextPrompt(sv SignVerifier) (Signed, error) {
	p, err := d.LastPrompt(sv)
	if err != nil {
		return Signed{}, err
	}
	source, a, err := d.LastAudit()
	if err != nil {
		return Signed{}, err
	}
	pp := p.NextUnsigned(fmt.Sprintf(`"auditor feedback (%s)": %s "original prompt:": %s`, source, a.Instruction, p.Data))
	return pp, pp.Sign(sv)
}

type Whole struct {
	thinkers map[string]Thinker
	// right        Thinker // a right brain interface TODO: tightent this interface (AKA Gemini)
	// left         Thinker // a left brain interface TODO: tighten this interface (AKA Claude)
	signVerifier SignVerifier
	heartbeat    time.Duration
	maxQueryTime time.Duration
	apispec      *openapi3.T
	queries      chan Query
	mu           big.Rat
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

func WithSpec(spec *openapi3.T) Option {
	return func(b *Whole) {
		b.apispec = spec
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

func (b *Whole) debateCycle(ctx context.Context, strategy []string, prompt Signed, genesis ...Signed) (Decision, error) {
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
	resp, err := think.Think(ctx, b.signVerifier, Request{T: prompt, G: genesis})
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
	return eval.Evaluate(ctx, b.signVerifier, resp)
}

func (b *Whole) Dream(ctx context.Context) {
	for _, think := range b.thinkers {
		hydration, err := think.Dream(ctx, b.apispec, b.signVerifier)
		if err != nil {
			log.Printf("failed sleep %s\n", err)
			continue
		}
		dreamdir := "./.dreams"
		now := time.Now()
		log.Printf("sleep gave: %#v\n", hydration)
		for _, h := range hydration {
			b, err := json.Marshal(h)
			if err != nil {
				log.Printf("could not marshal hydration document: %s", err)
				continue
			}
			err = os.MkdirAll(dreamdir, 0o700)
			if err != nil {
				log.Printf("could not store hydration document: %s", err)
			}
			err = os.WriteFile(dreamdir+`/`+think.Model()+now.UTC().Format(time.RFC3339)+`.dream`, b, 0o644)
			if err != nil {
				log.Printf("could not store hydration document: %s", err)
			}
		}
	}
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
		resp, err := think.Think(ctx, b.signVerifier, Request{T: Signed{Data: prompt}})
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
		dec, cycleErr = think.Evaluate(ctx, b.signVerifier, resp)
	default:
		dec, cycleErr = b.debateCycle(ctx, strategy, Signed{Data: prompt})
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

	decisions := Decisions{dec}
	for range MaxRecursions {
		p := decisions.GenesisPrompt()
		pp, err := decisions.NextPrompt(b.signVerifier)
		if err != nil {
			return decisions.Fold(b.signVerifier), err
		}
		dec, cycleErr := b.debateCycle(ctx, strategy, pp, p)
		if cycleErr != nil {
			return decisions.Fold(b.signVerifier), cycleErr
		}
		decisions = append(decisions, dec)
		if dec.IsError() != nil {
			return decisions.Fold(b.signVerifier), dec.IsError()
		}
		audits, err := parser.GetAudits(b.signVerifier, dec)
		if err != nil {
			return decisions.Fold(b.signVerifier), ErrNoConsensus
		}
		winner, ok := audits.WinningVerdict()
		if !ok {
			return decisions.Fold(b.signVerifier), ErrNoConsensus
		}
		dec.SetAudits(audits)
		if winner.Accepted() {
			return decisions.Fold(b.signVerifier), nil
		}
	}

	return decisions.Fold(b.signVerifier), ErrNoConsensus
}

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
	if targetDuration == 0 {
		targetDuration = 1 * time.Millisecond
	}
	ruminate := make(chan struct{})

	mincycles := int64(100)
	maxcycles := int64(100000)

	totalRumination := atomic.Int64{}

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
	prompts := atomic.Int64{}
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
									if took < time.Millisecond {
										took = time.Millisecond
									}
									velocity := float64(fuzzCycles) / float64(took.Nanoseconds())
									newTargetCycles := int(velocity * float64(targetDuration.Nanoseconds()))

									// ⚖️ [DAMPING]: Adjusted to 0.5/0.5 to filter out computation jitter
									fuzzCycles = int64(float64(fuzzCycles)*0.5 + float64(newTargetCycles)*0.5)

									fuzzCycles = max(fuzzCycles, mincycles)
									fuzzCycles = min(fuzzCycles, maxcycles)
								}
								fc.Store(fuzzCycles)
							}
							totalRumination.Add(took.Milliseconds())
							if totalRumination.Load() > SleepThreshold && prompts.Load() > 0 {
								b.Dream(appCtx)
								// create new context, rehydrate from core memories and latest hydration (or latest n)
								totalRumination.Store(0)
								prompts.Store(0)
							} else {
								log.Printf("staying awake for %v < %v\n", totalRumination.Load(), SleepThreshold)
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
					defer prompts.Add(1)
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
