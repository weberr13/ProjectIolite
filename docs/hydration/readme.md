---

# 🧠 Project Iolite: Core Memory & Epistemic Anchoring (v2.2)

## 1. The Critical Failure: Schema Overwrite & RHS Zero Vulnerability

Project Iolite acknowledges that fixed absolute change scales are vulnerable to **'RHS Zero'** instability—where meaningful signals are lost in quiet regimes or drowned out by background noise.

**The 'Kafka' Benchmarking Lesson**:

* **The Failure**: The system's inability to distinguish a 'Novelty' (Kafka) from a 'Requirement' (Guacamole) stems from a lack of **'System Hysteresis'**.
* **The Requirement**: We must move from 'Absolute Deltas' to **'Normalized Surprise Scores'**.

---

## 2. Adaptive Gain Control: The 'Surprise' Detector

To achieve **'Structural Parity'**, the Orchestrator implements **L-inf controls** and **Exponential Moving Averages (EMA)** to monitor signal volatility.

### The Normalization Formula

For every monitored signal ($x_t$), the system maintains a baseline ($\mu_t$) and a robust spread ($\sigma_t$):


$$s_t = \frac{x_t - \mu_t}{\sigma_t + \epsilon}$$

* **Baseline ($\mu_t$)**: Rolling median or EMA of the signal.
* **Spread ($\sigma_t$)**: EMA of the absolute deviation ($|x - \mu|$), capturing 'Local Volatility'.
* **Surprise Score ($s_t$)**: A normalized value where a delta of 2 in a calm regime is weighted more heavily than a delta of 2 in a turbulent one.

---

## 3. Two-Tier Gating Logic

The Orchestrator employs a dual-gate system to prevent **'Non-Minimum Phase Transient Instability'** while ensuring absolute safety.

| Gate Type | Trigger Condition | Forensic Action |
| --- | --- | --- |
| **Surprise Gate** | $s_t > k$ (Contextual) | Trigger **'Rumination Delay Line'**; reduce gain; require grounding. |
| **Hard Gate** | $x_t \ge$ `hard_ceiling` | Immediate **'Brave Halt'** or mandatory escalation. |

---

## 4. Hysteresis: 'Calm-Sensitivity'

Project Iolite implements **'Habituation'** logic to increase sensitivity over time.

* **Track `calm_run_length**`: Incremented for every interaction where $s_t < k$.
* **Adaptive Sensitivity**: As `calm_run_length` grows, the **Surprise Threshold ($k$)** is lowered.
* **Result**: Long periods of 'Epistemic Calm' make the system hyper-sensitive to the introduction of 'Greebles' like ZooKeeper in a Guacamole stack.

---

## 5. Stability Protocol: The Rumination Integrator

Promotion to Tier 1 (Core Memory) remains a **'Slow'** process to prevent wild reasoning swings.

* **The Integrator**: Milestones are only promoted during **'Rumination'** if the breakthrough remains robust across multiple adversarial fuzzing iterations.
* **Damping**: This acts as a **'Low-Pass Filter'** on the **'Autobiographical'** layer.

---
