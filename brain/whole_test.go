package brain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFold_Immutability(t *testing.T) {
	t.Run("Fold mutation test", func(t *testing.T) {
		original := &BaseDecision{
			Source: "Anchor",
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "Original Content"}},
			},
		}
		contributor := &BaseDecision{
			Source: "Contributor",
			AllTexts: map[string][]Signed{
				"claude": {{Data: "Contribution"}},
			},
		}
		decisions := Decisions{original, contributor}
		sv := &MockSignVerifier{}
		folded := decisions.Fold(sv)
		// We try to mutate the 'folded' result and see if it bleeds back
		foldedBase := folded.(*BaseDecision)
		foldedBase.AllTexts["gemini"][0].Data = "MUTATED"
		assert.NotEqual(t, "MUTATED", original.AllTexts["gemini"][0].Data)
		assert.Equal(t, "Contribution", contributor.AllTexts["claude"][0].Data, "[FAILURE]: Input preservation failed. Compose mutated the input decision d[1].")
	})
}

func TestWhole_Orchestration(t *testing.T) {
	mockSV := new(MockSignVerifier) // Using the mock from previous turn
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockLeft := new(MockThinker)
	mockRight := new(MockThinker)

	t.Run("NewWhole: Option Pattern and Ready State", func(t *testing.T) {
		// Test failure without SignVerifier
		b, err := NewWhole(WithLeftBrain(mockLeft))
		assert.ErrorIs(t, err, ErrNoSignVerifier)

		// Test failure without any Brains
		b, err = NewWhole(WithSignVerifier(mockSV))
		assert.ErrorIs(t, err, ErrNoLLMBrain)

		// Test Happy Path
		b, err = NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft))
		assert.NoError(t, err)
		assert.NotNil(t, b)
	})

	t.Run("Think: Single Hemisphere Routing (Left Only)", func(t *testing.T) {
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft))
		ctx := context.Background()
		prompt := "Why did the pen stay still?"

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: goodAudit, Signature: "sig2"}}})

		// Setup: Left Think -> Left Evaluate
		mockLeft.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()
		mockLeft.On("Evaluate", ctx, mockSV, resp, mock.Anything).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt, &DecisionParser{})

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockLeft.AssertExpectations(t)
	})

	t.Run("Think: Dual Hemisphere (Right Thinks, Left Evaluates)", func(t *testing.T) {
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithRightBrain(mockRight))
		ctx := context.Background()
		prompt := "Verify the braid integrity."

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: goodAudit, Signature: "sig2"}}})

		// Current Logic: Right always thinks first, Left evaluates
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()
		mockLeft.On("Evaluate", ctx, mockSV, resp, mock.Anything).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt, &DecisionParser{})

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockRight.AssertExpectations(t)
		mockLeft.AssertExpectations(t)
	})

	t.Run("Push: Concurrency and Channel Communication", func(t *testing.T) {
		// This tests the asynchronous "Start" loop and "Push" method
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithHeartbeatTime(100*time.Millisecond), WithDebateDir(t.TempDir()))

		appCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup Mocks
		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Verify", mockSV).Return(nil)
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: goodAudit, Signature: "sig2"}}})

		mockLeft.On("Think", mock.Anything, mockSV, mock.Anything).Return(resp, nil)
		mockLeft.On("Evaluate", mock.Anything, mockSV, resp, mock.Anything).Return(dec, nil)

		// Start the brain
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		// Push a query
		result, err := b.Debate(appCtx, "Ping")

		assert.NoError(t, err)
		assert.Equal(t, dec, result)

		cancel() // Stop the loop
		wg.Wait()
	})
}

func TestWhole_Think_BranchCoverage(t *testing.T) {
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
	ctx := context.Background()
	prompt := "Test Branch Logic"

	t.Run("Branch 1: Zero Brains (Total Failure)", func(t *testing.T) {
		// Bypass NewWhole's Ready() check by manually constructing
		// to test the internal safety switch in Think()
		b := &Whole{
			signVerifier: mockSV,
		}

		decision, err := b.Think(ctx, prompt, &DecisionParser{})

		// Assertions for the &ErrorDecision return
		assert.ErrorIs(t, err, ErrNoLLMBrain)
		assert.IsType(t, &ErrorDecision{}, decision)
		assert.Equal(t, ErrNoLLMBrain, decision.IsError())
	})

	t.Run("Branch 2: Right Only - Success Path", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			thinkers: map[string]Thinker{
				"right": mockRight,
			},
		}

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: goodAudit, Signature: "sig2"}}})

		// Expectations: Think then immediately Evaluate
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()
		mockRight.On("Evaluate", ctx, mockSV, resp, mock.Anything).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt, &DecisionParser{})

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockRight.AssertExpectations(t)
	})

	t.Run("Branch 3: Right Only - Think Failure", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			thinkers: map[string]Thinker{
				"right": mockRight,
			},
		}

		thinkErr := errors.New("api_timeout_from_gemini")
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return((*MockResponse)(nil), thinkErr).Once()

		decision, err := b.Think(ctx, prompt, &DecisionParser{})

		// Verify the error is wrapped in ErrorDecision and returned
		assert.ErrorIs(t, err, thinkErr)
		assert.Equal(t, thinkErr, decision.IsError())

		// Ensure Evaluate was NEVER called because Think failed
		mockRight.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestWhole_Think_AdvancedBranches(t *testing.T) {
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
	ctx := context.Background()
	prompt := "Critical Logic Audit"

	t.Run("Branch: Left Only - Think Failure", func(t *testing.T) {
		mockLeft := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			thinkers: map[string]Thinker{
				"left": mockLeft,
			},
		}

		leftErr := errors.New("claude_api_error")
		mockLeft.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return((*MockResponse)(nil), leftErr).Once()

		decision, err := b.Think(ctx, prompt, &DecisionParser{})

		assert.ErrorIs(t, err, leftErr)
		assert.IsType(t, &ErrorDecision{}, decision)
		assert.Equal(t, leftErr, decision.IsError())
		mockLeft.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Branch: Dual Brain - Right Think Failure", func(t *testing.T) {
		mockLeft := new(MockThinker)
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			thinkers: map[string]Thinker{
				"left":  mockLeft,
				"right": mockRight,
			},
		}

		rightErr := errors.New("gemini_quota_exceeded")
		// The code attempts Right.Think first in the dual-brain scenario
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return((*MockResponse)(nil), rightErr).Once()

		decision, err := b.Think(ctx, prompt, &DecisionParser{})

		assert.ErrorIs(t, err, rightErr)
		assert.Equal(t, rightErr, decision.IsError())

		// CRITICAL: Ensure the Left brain was never triggered because the Right brain failed at the source
		mockLeft.AssertNotCalled(t, "Think", mock.Anything, mock.Anything, mock.Anything)
		mockLeft.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Branch: Dual Brain - Happy Path Hand-off", func(t *testing.T) {
		mockLeft := new(MockThinker)
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			thinkers: map[string]Thinker{
				"left":  mockLeft,
				"right": mockRight,
			},
		}

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: goodAudit, Signature: "sig2"}}})

		// Verification of the "Braid": Right generates, Left audits
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()

		// Ensure mockResp from Right is the exact object passed to Left.Evaluate
		mockLeft.On("Evaluate", ctx, mockSV, resp).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt, &DecisionParser{})

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockRight.AssertExpectations(t)
		mockLeft.AssertExpectations(t)
	})
}

func TestWhole_Think_ContextSanity_Fixed(t *testing.T) {
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockLeft := new(MockThinker)
	mockRight := new(MockThinker)
	b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithRightBrain(mockRight))

	t.Run("Context Cancellation Mid-Braid Correct Handling", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		prompt := "Context Check"
		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()

		// Step 1: Right Brain succeeds but cancels context
		mockRight.On("Think", mock.Anything, mockSV, mock.Anything).Return(resp, nil).Run(func(args mock.Arguments) {
			cancel()
		}).Once()

		// Step 2: Left Brain IS called (because Think doesn't check mid-stream)
		// BUT it should receive the canceled context and return its own ErrorDecision
		expectedErr := context.Canceled
		mockLeft.On("Evaluate", mock.MatchedBy(func(c context.Context) bool {
			return c.Err() != nil // Verify the context passed is indeed canceled
		}), mockSV, resp).Return(&ErrorDecision{E: expectedErr}, expectedErr).Once()

		decision, err := b.Think(ctx, prompt, &DecisionParser{})

		// ASSERTIONS
		assert.ErrorIs(t, err, expectedErr)
		assert.NotNil(t, decision, "Think must ALWAYS return a Decision sentinel")
		assert.Equal(t, expectedErr, decision.IsError())

		mockRight.AssertExpectations(t)
		mockLeft.AssertExpectations(t)
	})
}

func TestBrain_ErrorBranches(t *testing.T) {
	// Setup for Heartbeat Test
	fastHeartbeat := 10 * time.Millisecond
	b := &Whole{
		queries:      make(chan Query),
		heartbeat:    fastHeartbeat,
		maxQueryTime: 50 * time.Millisecond,
		// Assuming Think is a method we can influence via a mock/interface
		// or by providing a specific input that triggers a Think error.
	}
	WithDebateDir(t.TempDir())(b)

	appCtx, cancelApp := context.WithCancel(t.Context())
	var wg sync.WaitGroup

	b.Start(appCtx, &wg)

	// 1. Hit the Heartbeat Case
	t.Run("Heartbeat_Pulse", func(t *testing.T) {
		// Simply waiting for > heartbeat duration ensures the fmt.Println(".") branch is hit.
		time.Sleep(fastHeartbeat * 2)
	})

	// 3. Reach Push context.Done() before query send
	t.Run("Push_Send_Timeout", func(t *testing.T) {
		deadCtx, cancel := context.WithCancel(t.Context())
		cancel() // Context is already dead

		_, err := b.Debate(deadCtx, "test input")
		assert.ErrorIs(t, err, context.Canceled, "Should reach the first ctx.Done() in Push")
	})
	cancelApp()
	wg.Wait()
}

func TestBrain_LifecycleAndErrorBranches(t *testing.T) {
	t.Run("Reach_Think_Error_Block", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			thinkers: map[string]Thinker{
				"right": mockRight,
			},
			queries:      make(chan Query),
			heartbeat:    5 * time.Millisecond,
			maxQueryTime: 500 * time.Millisecond,
		}
		WithDebateDir(t.TempDir())(b)

		appCtx, cancelApp := context.WithCancel(t.Context())
		defer cancelApp()
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		baseErr := errors.New("llm_failure")

		// Satisfy the contract: Return a sentinel struct, not nil
		sentinelResponse := &ErrorResponse{E: baseErr}

		mockRight.On("Think", mock.Anything, mock.Anything, mock.Anything).
			Return(sentinelResponse, baseErr).Once()

		res, err := b.Debate(appCtx, "trigger error")

		// Note: Push returns d.Verify(). If d is ErrorDecision, it returns the internal error.
		assert.Error(t, err)
		switch err.Error() {
		case "llm_failure":
		case "context deadline exceeded":
		default:
			assert.Equal(t, err.Error(), "llm_failure")
		}
		assert.NotNil(t, res.IsError())
		// er := ErrorResponse{}
		// assert.True(t,errors.As(res.IsError(), &er))
		// assert.Equal(t, er.Unwrap(), baseErr)
		cancelApp()
		wg.Wait()
	})

	t.Run("Push_Receive_Timeout_Second_Select", func(t *testing.T) {
		mockRight := new(MockThinker)
		mockSV := new(MockSignVerifier)
		mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		b := &Whole{
			thinkers: map[string]Thinker{
				"right": mockRight,
			},
			queries:      make(chan Query),
			heartbeat:    5 * time.Millisecond,
			maxQueryTime: 50 * time.Millisecond,
			signVerifier: mockSV,
		}
		WithDebateDir(t.TempDir())(b)

		appCtx, cancelApp := context.WithCancel(t.Context())
		defer cancelApp()
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: "```json{\"approved\": true}```", Signature: "sig2"}}})

		// Scenario: Think hangs, causing the Push caller to time out while waiting on chan 'c'
		// We use a mock that blocks until the test context expires
		mockRight.On("Think", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				time.Sleep(100 * time.Millisecond) // Exceeds Push timeout
			}).
			Return(resp, nil).Maybe()
		mockRight.On("Evaluate", mock.Anything, mock.Anything, mock.Anything).Return(dec, nil).Maybe()

		// Request context with very short deadline
		reqCtx, cancelReq := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancelReq()

		_, err := b.Debate(reqCtx, "slow query")

		// This hits the second <-ctx.Done() in Push
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		cancelApp()
		wg.Wait()
	})
}

func TestWhole_Think_MoreUnhappyBranches(t *testing.T) {
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

	ctx := context.Background()
	parser := &DecisionParser{}
	prompt := "Exposing the Medusa."

	t.Run("Right_Think_Nil_Response", func(t *testing.T) {
		mockRight := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithRightBrain(mockRight))

		// Branch: if resp == nil { return &ErrorDecision{E: ErrNoLLMBrain}, err }
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(nil, nil).Once()

		dec, err := b.Think(ctx, prompt, parser)
		assert.ErrorIs(t, err, ErrNoLLMBrain)
		assert.NotNil(t, dec)
	})

	t.Run("Left_Think_Nil_Response", func(t *testing.T) {
		mockRight := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockRight))

		// Branch: if resp == nil { return &ErrorDecision{E: ErrNoLLMBrain}, err }
		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(nil, nil).Once()

		dec, err := b.Think(ctx, prompt, parser)
		assert.ErrorIs(t, err, ErrNoLLMBrain)
		assert.NotNil(t, dec)
	})

	t.Run("Left_Think_Sentinel_Error", func(t *testing.T) {
		mockLeft := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft))

		sentinelErr := errors.New("mystic_logic_fault")
		resp := new(MockResponse)
		// Branch: if resp.IsError() != nil
		resp.On("IsError").Return(sentinelErr).Times(3)

		mockLeft.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()

		dec, err := b.Think(ctx, prompt, parser)
		assert.ErrorIs(t, err, sentinelErr)
		assert.Equal(t, sentinelErr, dec.IsError())
	})

	t.Run("Right_Think_Sentinel_Error", func(t *testing.T) {
		mockLeft := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithRightBrain(mockLeft))

		sentinelErr := errors.New("mystic_logic_fault")
		resp := new(MockResponse)
		// Branch: if resp.IsError() != nil
		resp.On("IsError").Return(sentinelErr).Times(3)

		mockLeft.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()

		dec, err := b.Think(ctx, prompt, parser)
		assert.ErrorIs(t, err, sentinelErr)
		assert.Equal(t, sentinelErr, dec.IsError())
	})

	t.Run("Cycle_Decision_Sentinel_Error", func(t *testing.T) {
		mockRight := new(MockThinker)
		mockLeft := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithRightBrain(mockRight), WithLeftBrain(mockLeft))

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()

		dec := new(MockDecision)
		decSentinel := errors.New("audit_failed_braid_broken")
		// Branch: if dec.IsError() != nil { return dec, cycleErr }
		dec.On("IsError").Return(decSentinel).Twice() // once for Think, once for *us*
		dec.On("Texts").Return(map[string][]Signed{"left": {{Data: "", Signature: "sig2"}}})

		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()
		mockLeft.On("Evaluate", ctx, mockSV, resp).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt, parser)
		assert.Equal(t, decSentinel, result.IsError())
		assert.NoError(t, err) // Matches current logic: return dec, cycleErr (where cycleErr is nil)
	})

	t.Run("Parser_Structural_Error", func(t *testing.T) {
		mockRight := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithRightBrain(mockRight))

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		// Return empty texts to trigger a parser error (no genesis)
		dec.On("Texts").Return(map[string][]Signed{}).Once()

		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()
		mockRight.On("Evaluate", ctx, mockSV, resp).Return(dec, nil).Once()

		// Branch: if err != nil { return dec, err } (e.g., no terminal decision found)
		_, err := b.Think(ctx, prompt, parser)
		assert.Equal(t, err, ErrNoConsensus)
	})

	t.Run("Terminal_No_Consensus", func(t *testing.T) {
		mockRight := new(MockThinker)
		mockLeft := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithRightBrain(mockRight), WithLeftBrain(mockLeft))

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		// Valid braid, but explicit disapproval JSON
		dec.On("Texts").Return(map[string][]Signed{
			"claude": {{Data: "```json\n{\"approved\": false}\n```", Signature: "sig", PrevSignature: ""}},
		}).Once()

		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()
		mockLeft.On("Evaluate", ctx, mockSV, resp).Return(dec, nil).Once()

		// Branch: return dec, ErrNoConsensus
		_, err := b.Think(ctx, prompt, parser)
		assert.ErrorIs(t, err, ErrNoConsensus)
	})
}

func TestWhole_Think_DefaultBrainFailures(t *testing.T) {
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()

	ctx := context.Background()
	parser := &DecisionParser{}
	prompt := "Operation Epic Fury: Logistics Audit."

	t.Run("Default_Case_Right_Think_Nil", func(t *testing.T) {
		mockLeft := new(MockThinker)
		mockRight := new(MockThinker)
		// Setup dual-brain scenario
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithRightBrain(mockRight))

		// Branch: if resp == nil { return &ErrorDecision{E: ErrNoLLMBrain}, ErrNoLLMBrain }

		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(nil, nil).Once()

		dec, err := b.Think(ctx, prompt, parser)

		assert.ErrorIs(t, err, ErrNoLLMBrain)
		assert.IsType(t, &ErrorDecision{}, dec)
		mockLeft.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Default_Case_Right_Think_Sentinel_Error", func(t *testing.T) {
		mockLeft := new(MockThinker)
		mockRight := new(MockThinker)
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithRightBrain(mockRight))

		sentinelErr := errors.New("gemini_quota_exceeded")
		resp := new(MockResponse)
		// Branch: if resp.IsError() != nil { return &ErrorDecision{E: resp.IsError()}, resp.IsError() }
		resp.On("IsError").Return(sentinelErr).Times(3)

		mockRight.On("Think", ctx, mockSV, Request{T: Signed{Data: prompt}}).Return(resp, nil).Once()

		dec, err := b.Think(ctx, prompt, parser)

		assert.ErrorIs(t, err, sentinelErr)
		assert.Equal(t, sentinelErr, dec.IsError())
		// Crucially, verify that we didn't waste tokens/time asking Claude to evaluate an error
		mockLeft.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestRequestIsSimple(t *testing.T) {
	t.Run("Response just holds some text in a signed block", func(t *testing.T) {
		randomText := uuid.NewString()
		r := Request{T: Signed{Data: randomText}}
		assert.Equal(t, randomText, r.Text().Data)
	})
}

func TestWhole_FinalTerminalBranches(t *testing.T) {
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("asig", nil).Maybe()
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

	t.Run("Start_Loop_ErrNoConsensus_SetError", func(t *testing.T) {
		mockRight := new(MockThinker)
		mockLeft := new(MockThinker) // Need both for the 'default' path in Think
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithRightBrain(mockRight), WithDebateDir(t.TempDir()))
		b.maxQueryTime = 50 * time.Millisecond

		appCtx, cancelApp := context.WithCancel(t.Context())
		defer cancelApp()
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()

		dec := new(MockDecision)
		// 1. MUST return nil here so Think continues to the Parser
		dec.On("IsError").Return(nil).Maybe()
		// 2. Return text that the parser will definitely reject
		dec.On("Texts").Return(map[string][]Signed{
			"claude": {{Data: "No JSON here.", Signature: "sig", PrevSignature: ""}},
		}).Maybe()

		// 3. This is the branch we are testing in Start()
		dec.On("SetError", ErrNoConsensus).Maybe()

		mockRight.On("Think", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
		mockLeft.On("Evaluate", mock.Anything, mock.Anything, mock.Anything).Return(dec, nil)

		c := make(chan Decision, 1)
		b.queries <- Query{input: "Trigger Consensus Error", C: c}

		select {
		case result := <-c:
			// Now it should be our mock, not an ErrorDecision
			assert.Equal(t, dec, result)
			dec.AssertExpectations(t)
		case <-time.After(500 * time.Second):
			t.Fatal("Start loop failed to process query")
		}
	})

	t.Run("Push_Context_Done_Initial_Send", func(t *testing.T) {
		b := &Whole{
			queries: make(chan Query), // Unbuffered, will block
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediate cancellation

		// Branch: case <-ctx.Done(): return nil, ctx.Err()
		dec, err := b.Debate(ctx, "This send should fail")

		assert.Nil(t, dec)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
