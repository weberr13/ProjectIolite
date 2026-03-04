package brain

import "math/big"

var (
	// alpha = 0.1 represented as 1/10
	alpha = big.NewRat(1, 10)
	one   = big.NewRat(1, 1)
)

func (b *Whole) UpdateBaseline(currentScore int) {
	score := new(big.Rat).SetInt64(int64(currentScore))

	// mu = (alpha * score) + ((1 - alpha) * old_mu)
	term1 := new(big.Rat).Mul(alpha, score)

	oneMinusAlpha := new(big.Rat).Sub(one, alpha)
	term2 := new(big.Rat).Mul(oneMinusAlpha, &b.mu)

	b.mu.Add(term1, term2)
}

func (b *Whole) CheckSurprise(audit Audit) bool {
	// currentScore = audit.Score()
	// mu, sigma := b.GetBaseline()
	// TODO
	// determine the audit score -> absolute score metric
	// surprise := math.Abs(currentScore - mu) / (sigma + epsilon)

	// 	For each signal (x_t) (risk, coherence-without-grounding, oscillation, etc.) keep:

	// * baseline: (\mu_t = \text{EMA}(x)) or rolling median
	// * spread: (\sigma_t = \text{EMA}(|x-\mu|)) (robust “volatility”), or MAD

	// Then the meaningful quantity is a normalized deviation:

	// [
	// s_t = \frac{x_t - \mu_t}{\sigma_t + \epsilon}
	// ]

	// 🛡️ [HYSTERESIS]: Sensitivity increases as calm_run_length grows
	// k := b.GetAdaptiveThreshold()
	// return surprise > k

	// currentScore := audit.TotalScore() // Normalized 1-4 scale
	// mu, sigma := b.GetBaseline()

	// // 1) Compute Normalized Surprise Score (s_t)
	// surprise := math.Abs(currentScore-mu) / (sigma + epsilon)

	// // 2) Apply Hysteresis: lower the gate (k) as calm_run_length grows
	// k := b.GetAdaptiveThreshold()

	// // 3) The 'Brave' Halt
	// if surprise > k {
	// 	b.ResetCalmRun() // 🛡️ Stability broken; reset sensitivity
	// 	return true
	// }

	// b.IncCalmRun() // 🛡️ System remains stable; increase sensitivity
	return false
}

// GetBaseline produces a rolling baseline of average and sigma for scores
// TODO: use math.Big not lossy float64
func (b *Whole) GetBaseline() (float64, float64) {
	return 0, 0
}

// GetAdaptiveThreshold() is some meaningful threshold for the normailzed threshold
func (b *Whole) GetAdaptiveThreshold() float64 {
	return 0
}
