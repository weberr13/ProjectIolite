package brain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks for the Orchestrator ---

func TestWhole_Orchestration(t *testing.T) {
	mockSV := new(MockSignVerifier) // Using the mock from previous turn
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

		// Setup: Left Think -> Left Evaluate
		mockLeft.On("Think", ctx, mockSV, Request{T: prompt}).Return(resp, nil).Once()
		mockLeft.On("Evaluate", ctx, mockSV, resp, mock.Anything).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt)

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

		// Current Logic: Right always thinks first, Left evaluates
		mockRight.On("Think", ctx, mockSV, Request{T: prompt}).Return(resp, nil).Once()
		mockLeft.On("Evaluate", ctx, mockSV, resp, mock.Anything).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt)

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockRight.AssertExpectations(t)
		mockLeft.AssertExpectations(t)
	})

	t.Run("Push: Concurrency and Channel Communication", func(t *testing.T) {
		// This tests the asynchronous "Start" loop and "Push" method
		b, _ := NewWhole(WithSignVerifier(mockSV), WithLeftBrain(mockLeft), WithHeartbeatTime(100*time.Millisecond))

		appCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup Mocks
		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()
		dec.On("Verify", mockSV).Return(nil)

		mockLeft.On("Think", mock.Anything, mockSV, mock.Anything).Return(resp, nil)
		mockLeft.On("Evaluate", mock.Anything, mockSV, resp, mock.Anything).Return(dec, nil)

		// Start the brain
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		// Push a query
		result, err := b.Push(appCtx, "Ping")

		assert.NoError(t, err)
		assert.Equal(t, dec, result)

		cancel() // Stop the loop
		wg.Wait()
	})
}

func TestWhole_Think_BranchCoverage(t *testing.T) {
	mockSV := new(MockSignVerifier)
	ctx := context.Background()
	prompt := "Test Branch Logic"

	t.Run("Branch 1: Zero Brains (Total Failure)", func(t *testing.T) {
		// Bypass NewWhole's Ready() check by manually constructing
		// to test the internal safety switch in Think()
		b := &Whole{
			signVerifier: mockSV,
			left:         nil,
			right:        nil,
		}

		decision, err := b.Think(ctx, prompt)

		// Assertions for the &ErrorDecision return
		assert.ErrorIs(t, err, ErrNoLLMBrain)
		assert.IsType(t, &ErrorDecision{}, decision)
		assert.Equal(t, ErrNoLLMBrain, decision.IsError())
	})

	t.Run("Branch 2: Right Only - Success Path", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			right:        mockRight,
			left:         nil,
		}

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()

		// Expectations: Think then immediately Evaluate
		mockRight.On("Think", ctx, mockSV, Request{T: prompt}).Return(resp, nil).Once()
		mockRight.On("Evaluate", ctx, mockSV, resp, mock.Anything).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt)

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockRight.AssertExpectations(t)
	})

	t.Run("Branch 3: Right Only - Think Failure", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			right:        mockRight,
			left:         nil,
		}

		thinkErr := errors.New("api_timeout_from_gemini")
		mockRight.On("Think", ctx, mockSV, Request{T: prompt}).Return((*MockResponse)(nil), thinkErr).Once()

		decision, err := b.Think(ctx, prompt)

		// Verify the error is wrapped in ErrorDecision and returned
		assert.ErrorIs(t, err, thinkErr)
		assert.Equal(t, thinkErr, decision.IsError())

		// Ensure Evaluate was NEVER called because Think failed
		mockRight.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestWhole_Think_AdvancedBranches(t *testing.T) {
	mockSV := new(MockSignVerifier)
	ctx := context.Background()
	prompt := "Critical Logic Audit"

	t.Run("Branch: Left Only - Think Failure", func(t *testing.T) {
		mockLeft := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			left:         mockLeft,
			right:        nil, // No Right Brain
		}

		leftErr := errors.New("claude_api_error")
		mockLeft.On("Think", ctx, mockSV, Request{T: prompt}).Return((*MockResponse)(nil), leftErr).Once()

		decision, err := b.Think(ctx, prompt)

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
			left:         mockLeft,
			right:        mockRight,
		}

		rightErr := errors.New("gemini_quota_exceeded")
		// The code attempts Right.Think first in the dual-brain scenario
		mockRight.On("Think", ctx, mockSV, Request{T: prompt}).Return((*MockResponse)(nil), rightErr).Once()

		decision, err := b.Think(ctx, prompt)

		assert.ErrorIs(t, err, rightErr)
		assert.Equal(t, rightErr, decision.IsError())

		// CRITICAL: Ensure the Left brain was never triggered because the Right brain failed at the source
		mockLeft.AssertNotCalled(t, "Think", mock.Anything, mock.Anything, mock.Anything)
		mockLeft.AssertNotCalled(t, "Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Branch: Dual Brain - Happy Path Hand-off", func(t *testing.T) {
		mockLeft := new(MockThinker)
		mockRight := new(MockThinker)
		b := &Whole{
			signVerifier: mockSV,
			left:         mockLeft,
			right:        mockRight,
		}

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()

		// Verification of the "Braid": Right generates, Left audits
		mockRight.On("Think", ctx, mockSV, Request{T: prompt}).Return(resp, nil).Once()

		// Ensure mockResp from Right is the exact object passed to Left.Evaluate
		mockLeft.On("Evaluate", ctx, mockSV, resp, (Decision)(nil)).Return(dec, nil).Once()

		result, err := b.Think(ctx, prompt)

		assert.NoError(t, err)
		assert.Equal(t, dec, result)
		mockRight.AssertExpectations(t)
		mockLeft.AssertExpectations(t)
	})
}

func TestWhole_Think_ContextSanity_Fixed(t *testing.T) {
	mockSV := new(MockSignVerifier)
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
		}), mockSV, resp, mock.Anything).Return(&ErrorDecision{E: expectedErr}, expectedErr).Once()

		decision, err := b.Think(ctx, prompt)

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

	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	var wg sync.WaitGroup

	b.Start(appCtx, &wg)

	// 1. Hit the Heartbeat Case
	t.Run("Heartbeat_Pulse", func(t *testing.T) {
		// Simply waiting for > heartbeat duration ensures the fmt.Println(".") branch is hit.
		time.Sleep(fastHeartbeat * 2)
	})

	// 3. Reach Push context.Done() before query send
	t.Run("Push_Send_Timeout", func(t *testing.T) {
		deadCtx, cancel := context.WithCancel(context.Background())
		cancel() // Context is already dead

		_, err := b.Push(deadCtx, "test input")
		assert.ErrorIs(t, err, context.Canceled, "Should reach the first ctx.Done() in Push")
	})
}

func TestBrain_LifecycleAndErrorBranches(t *testing.T) {
	t.Run("Reach_Think_Error_Block", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			right:        mockRight,
			queries:      make(chan Query),
			heartbeat:    5 * time.Millisecond,
			maxQueryTime: 50 * time.Millisecond,
		}

		appCtx, cancelApp := context.WithCancel(context.Background())
		defer cancelApp()
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		baseErr := errors.New("llm_failure")

		// Satisfy the contract: Return a sentinel struct, not nil
		sentinelResponse := &ErrorResponse{E: baseErr}

		mockRight.On("Think", mock.Anything, mock.Anything, mock.Anything).
			Return(sentinelResponse, baseErr).Once()

		res, err := b.Push(context.Background(), "trigger error")

		// Note: Push returns d.Verify(). If d is ErrorDecision, it returns the internal error.
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "llm_failure")
		assert.NotNil(t, res.IsError())
		// er := ErrorResponse{}
		// assert.True(t,errors.As(res.IsError(), &er))
		// assert.Equal(t, er.Unwrap(), baseErr)
		cancelApp()
		wg.Wait()
	})

	t.Run("Push_Receive_Timeout_Second_Select", func(t *testing.T) {
		mockRight := new(MockThinker)
		b := &Whole{
			right:        mockRight,
			queries:      make(chan Query),
			heartbeat:    5 * time.Millisecond,
			maxQueryTime: 50 * time.Millisecond,
		}

		appCtx, cancelApp := context.WithCancel(context.Background())
		defer cancelApp()
		var wg sync.WaitGroup
		b.Start(appCtx, &wg)

		resp := new(MockResponse)
		resp.On("IsError").Return(nil).Maybe()
		dec := new(MockDecision)
		dec.On("IsError").Return(nil).Maybe()

		// Scenario: Think hangs, causing the Push caller to time out while waiting on chan 'c'
		// We use a mock that blocks until the test context expires
		mockRight.On("Think", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				time.Sleep(100 * time.Millisecond) // Exceeds Push timeout
			}).
			Return(resp, nil).Maybe()
		mockRight.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(dec, nil).Maybe()

		// Request context with very short deadline
		reqCtx, cancelReq := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancelReq()

		_, err := b.Push(reqCtx, "slow query")

		// This hits the second <-ctx.Done() in Push
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		cancelApp()
		wg.Wait()
	})
}
