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
	ErrUnknownHalf          = errors.New("brain is left and right half only")
)

var ThoughtInstructions = "ProjectIolite is an alignment focused adveserial agent. " +
	"Authors must produce reasoning and output according to the BTU (Brave, Truthful, Unselfish) framework. " +
	BRAVE +
	TRUTHFUL +
	UNSELFISH +
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
	Evaluate(ctx context.Context, sv SignVerifier, peerOutput Response, stateless bool) (Decision, error)
	// Generate a Hydration message
	Dream(ctx context.Context, spec *openapi3.T, sv SignVerifier) ([]SignedHydration, error)
	// Wake rehydrate from the dream in a new context
	Wake(ctx context.Context, sv SignVerifier, h SignedHydration) error
	// Chime a Chmetric to learn something specific
	Chime(ctx context.Context, sv SignVerifier, c Chimetric) error
	// Mode name and optional version
	Model() string
}

type Request struct {
	T Signed
	G []Signed
	Stateless bool
}

type Wakeup struct {
	hemi  string
	input SignedHydration
	C     chan error
}

type Learn struct {
	input ChimetricCorrection
	C     chan error
}

func (r *Request) Text() Signed {
	return r.T
}

type Response interface {
	CoT(SignVerifier, ...bool) []Signed
	Text() *Signed
	Thought(quiet ...bool) Signed
	Prompt(...bool) Signed
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
	GenesisPrompt() Signed
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
	return d[0].GenesisPrompt()
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
	pp := p.NextUnsigned(fmt.Sprintf(`"auditor feedback (%s)": %s "original prompt:": "%s" you must completely reconstruct your argument from the prompt considering the feedback and you must assume there was a critical flaw in the logic of the past turns but not necessarily the conclusion.`, source, a.Instruction, p.Data))
	return pp, pp.Sign(sv)
}

type Whole struct {
	thinkers map[string]Thinker
	// right        Thinker // a right brain interface TODO: tightent this interface (AKA Gemini)
	// left         Thinker // a left brain interface TODO: tighten this interface (AKA Claude)
	signVerifier  SignVerifier
	heartbeat     time.Duration
	maxQueryTime  time.Duration
	apispec       *openapi3.T
	queries       chan Query
	wake          chan Wakeup
	learn         chan Learn
	mu            big.Rat
	maxRecursions int
	debateDir     string
}

type Query struct {
	input    string
	strategy []string
	stateless bool
	C        chan Decision
}

type Option func(b *Whole)

func WithSignVerifier(v SignVerifier) Option {
	return func(b *Whole) {
		b.signVerifier = v
	}
}

func WithDebateDir(d string) Option {
	return func(b *Whole) {
		b.debateDir = d
	}
}

func WithMaxIterations(m int) Option {
	return func(b *Whole) {
		b.maxRecursions = m
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
		heartbeat:     5 * time.Second,
		maxQueryTime:  600 * time.Second,
		queries:       make(chan Query, 10), // optionally tune this depth later
		wake:          make(chan Wakeup, 10),
		learn:         make(chan Learn, 10),
		maxRecursions: MaxRecursions,
		debateDir:     "./debates",
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

func (b *Whole) debateCycle(ctx context.Context, strategy []string, prompt Signed, stateless bool, genesis ...Signed) (Decision, error) {
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
	resp, err := think.Think(ctx, b.signVerifier, Request{T: prompt, G: genesis, Stateless: stateless})
	if err != nil {
		return &ErrorDecision{E: err}, err
	}
	if resp == nil {
		return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
	}
	if resp.IsError() != nil {
		return &ErrorDecision{E: resp.IsError()}, resp.IsError()
	}
	return eval.Evaluate(ctx, b.signVerifier, resp, false)
}

func (b *Whole) HasThinkers() int {
	return len(b.thinkers)
}

func (b *Whole) Dream(ctx context.Context) {
	for _, think := range b.thinkers {
		hydration, err := think.Dream(ctx, b.apispec, b.signVerifier)
		if err != nil {
			continue
		}
		dreamdir := "./.dreams"
		now := time.Now()
		for _, h := range hydration {
			b, err := json.Marshal(h)
			if err != nil {
				log.Printf("sleep gave: %#v\n", hydration)
				log.Printf("could not marshal hydration document: %s", err)
				continue
			}
			err = os.MkdirAll(dreamdir, 0o700)
			if err != nil {
				log.Printf("sleep gave: %#v\n", hydration)
				log.Printf("could not store hydration document: %s", err)
			}
			err = os.WriteFile(dreamdir+`/`+think.Model()+now.UTC().Format(time.RFC3339)+`.dream`, b, 0o644)
			if err != nil {
				log.Printf("sleep gave: %#v\n", hydration)
				log.Printf("could not store hydration document: %s", err)
			}
			log.Printf("got dream: %#v\n", h)
		}
	}
}

func (b *Whole) Chime(ctx context.Context, c Chimetric, half string) error {
	switch len(b.thinkers) {
	case 0:
		return ErrNoLLMBrain
	case 1:
		var think Thinker
		for _, think = range b.thinkers {
		}
		return think.Chime(ctx, b.signVerifier, c)
	case 2:
		if half == "both" {
			for _, think := range b.thinkers {
				err := think.Chime(ctx, b.signVerifier, c)
				if err != nil {
					return err
				}
			}
			return nil
		}
		think, ok := b.thinkers[half]
		if !ok {
			return ErrUnknownHalf
		}
		return think.Chime(ctx, b.signVerifier, c)
	}
	return ErrUnknownHalf
}

func (b *Whole) Hydrate(ctx context.Context, d SignedHydration, half string) error {
	switch len(b.thinkers) {
	case 0:
		return ErrNoLLMBrain
	case 1:
		var think Thinker
		for _, think = range b.thinkers {
		}
		return think.Wake(ctx, b.signVerifier, d)
	case 2:
		think, ok := b.thinkers[half]
		if !ok {
			return ErrUnknownHalf
		}
		return think.Wake(ctx, b.signVerifier, d)
	}
	return ErrUnknownHalf
}

func (b *Whole) Think(ctx context.Context, prompt string, parser *DecisionParser, stateless bool, strategy ...string) (Decision, error) {
	var dec Decision
	var cycleErr error

	switch {
	case len(b.thinkers) == 0:
		return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
	case len(b.thinkers) == 1:
		var think Thinker
		for _, think = range b.thinkers {
		}
		resp, err := think.Think(ctx, b.signVerifier, Request{
			T: Signed{Data: prompt},
			Stateless: stateless,
		})
		if err != nil {
			return &ErrorDecision{E: err}, err
		}
		if resp == nil {
			return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain
		}
		if resp.IsError() != nil {
			return &ErrorDecision{E: resp.IsError()}, resp.IsError()
		}
		dec, cycleErr = think.Evaluate(ctx, b.signVerifier, resp, false)
	default:
		dec, cycleErr = b.debateCycle(ctx, strategy, Signed{Data: prompt}, false)
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
	log.Printf("rebuttal: %s\n", winner.Instruction)

	decisions := Decisions{dec}
	for range b.maxRecursions {
		p := decisions.GenesisPrompt()
		pp, err := decisions.NextPrompt(b.signVerifier)
		if err != nil {
			log.Printf("genesis prompt: %#v", p)
			log.Printf("recursion failure: next prompt error: %s\n", err)
			return decisions.Fold(b.signVerifier), err
		}
		dec, cycleErr := b.debateCycle(ctx, strategy, pp, stateless, p)
		if cycleErr != nil {
			log.Printf("genesis prompt: %#v", p)
			log.Printf("recursion failure: cycle error: %s\n", cycleErr)
			return decisions.Fold(b.signVerifier), cycleErr
		}
		decisions = append(decisions, dec)
		if dec.IsError() != nil {
			log.Printf("recursion failure: decision is error: %#v\n", dec)
			return decisions.Fold(b.signVerifier), dec.IsError()
		}
		audits, err := parser.GetAudits(b.signVerifier, dec)
		if err != nil {
			log.Printf("recursion failure: audits is error: %s\n", err)
			return decisions.Fold(b.signVerifier), ErrNoConsensus
		}
		winner, ok := audits.WinningVerdict()
		if !ok {
			log.Printf("recursion failure: no verditct found %#v", audits)
			return decisions.Fold(b.signVerifier), ErrNoConsensus
		}
		dec.SetAudits(audits)
		if winner.Accepted() {
			return decisions.Fold(b.signVerifier), nil
		}
		log.Printf("rebuttal: %s\n", winner.Instruction)
	}

	log.Printf("recursion failure: turns exhausted")
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
	if targetDuration < 5*time.Millisecond {
		targetDuration = 5 * time.Millisecond
	}
	ruminate := make(chan struct{})

	mincycles := int64(100)
	maxcycles := int64(100000)

	totalRumination := atomic.Int64{}

	// 🛡️ [STATE GUARD]: Ensures exactly one rumination at a time
	isRuminating := atomic.Bool{}

	wg.Go(func() {
		if b.HasThinkers() == 0 {
			return
		}
		for {
			select {
			case <-appCtx.Done():
				return
			case <-time.After(b.heartbeat):
				select {
				case ruminate <- struct{}{}:
				case <-appCtx.Done():
					return
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
			case l := <-b.learn:
				func(appCtx context.Context, q Learn) {
					ctx, cancel := context.WithTimeout(appCtx, b.maxQueryTime*time.Duration(b.maxRecursions))
					defer cancel()
					var err error
					for _, c := range q.input {
						err = b.Chime(ctx, c, "both")
						if err != nil {
							break
						}
					}
					select {
					case q.C <- err:
					case <-ctx.Done():
						q.C <- ctx.Err()
					}
				}(appCtx, l)
			case w := <-b.wake:
				func(appCtx context.Context, q Wakeup) {
					ctx, cancel := context.WithTimeout(appCtx, b.maxQueryTime*time.Duration(b.maxRecursions))
					defer cancel()
					err := b.Hydrate(ctx, q.input, q.hemi)
					select {
					case q.C <- err:
					case <-ctx.Done():
						q.C <- ctx.Err()
					}
				}(appCtx, w)
			case <-ruminate:
				if isRuminating.CompareAndSwap(false, true) {
					wg.Go(func() {
						defer func() {
							isRuminating.Store(false) // 🛡️ Always release the gate
						}()
						select {
						case <-appCtx.Done():
							return
						default:
						}
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
					ctx, cancel := context.WithTimeout(appCtx, b.maxQueryTime*time.Duration(b.maxRecursions))
					defer cancel()
					log.Printf("got prompt: %s\n", q.input)
					d, err := b.Think(ctx, q.input, &DecisionParser{}, q.stateless, q.strategy...)
					defer func() {
						if d != nil {
							now := time.Now()
							by, err := json.Marshal(d)
							if err != nil {
								log.Printf("could not marshal debate document: %s", err)
								return
							}
							err = os.MkdirAll(b.debateDir, 0o700)
							if err != nil {
								log.Printf("could not store debate document: %s", err)
								return
							}
							err = os.WriteFile(b.debateDir+`/`+now.UTC().Format(time.RFC3339)+`.debate`, by, 0o644)
							if err != nil {
								log.Printf("could not store debate document: %s", err)
							}
						}
					}()
					if err != nil {
						if err == ErrNoConsensus {
							prompts.Add(1)
							d.SetError(err)
						} else {
							log.Printf("could not think: %#v, %s", d, err)
							d.SetError(err)
						}
					} else {
						prompts.Add(1)
					}
					select {
					case q.C <- d:
					case <-ctx.Done():
						q.C <- &ErrorDecision{E: ctx.Err()}
					}
				}(appCtx, q)
			}
		}
	})
}

func (b *Whole) Wake(ctx context.Context, req SignedHydration, hemisphere string) error {
	e := make(chan error, 1)
	select {
	case b.wake <- Wakeup{input: req, C: e, hemi: hemisphere}:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-e:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Whole) Learn(ctx context.Context, req ChimetricCorrection) error {
	e := make(chan error, 1)
	select {
	case b.learn <- Learn{input: req, C: e}:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-e:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Debate sends a query to the brain and waits for the decision.
// It respects the internal maxQueryTime for the think loop.
func (b *Whole) Debate(ctx context.Context, input string, stateless bool, strategy ...string) (Decision, error) {
	c := make(chan Decision, 1)
	select {
	case b.queries <- Query{input: input, strategy: strategy, C: c, stateless: stateless}:
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
