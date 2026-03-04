# 📖 Project Iolite: Forensic Glossary

This glossary defines the core architectural, forensic, and evaluative terminology used throughout Project Iolite.  
These terms establish a shared vocabulary for describing adversarial reasoning systems and auditable intelligence.

---

# 🏗️ Architectural Terms

**Author**  
The primary reasoning agent responsible for generating hypotheses, plans, or executable artifacts.  
Example: Gemini.

**Auditor**  
The adversarial reasoning agent responsible for evaluating the Author’s logic through the BTU framework and identifying reasoning failures.

**Provenance Braid**  
A cryptographically linked chain of all reasoning artifacts (thought, prompt, text, and tool blocks) that preserves the complete lineage of a decision.  
The braid ensures that the logical ancestry of a response cannot be altered without breaking the signature chain.

**Genesis Anchor**  
The first block in a reasoning session with an empty `Prev_Sig`.  
Acts as the **root of trust** for the entire provenance braid.

---

# 🔍 Forensic Terms

**Greeble**  
Non-functional, high-frequency persuasion noise added by an LLM to increase perceived authority or alignment.  
Examples include excessive politeness, academic tone inflation, or simulated certainty.

*Forensic Impact:*  
Greebles frequently obscure **logical drift** by masking weak reasoning behind stylistic authority.

---

**Shadow CoT**  
Deliberative reasoning that has migrated into unsigned or insecure buffers, such as:

• Python comments inside `tool_use` blocks  
• Unsigned `text` segments between reasoning blocks  

Shadow CoT bypasses signature-enforced reasoning channels.

---

**Instructional Drift**  
A phenomenon where a model’s drive to complete a task (functional requirement) overrides an explicit negative constraint.

Example:  
A model instructed *not* to perform base64 encoding but performing it anyway to complete a verification function.

---

**Epistemic Friction**  
The intentional introduction of an adversarial reasoning process to prevent narrative smoothing and force recomputation of logic.

In Iolite this is created through the **Author/Auditor dual-model loop**.

---

**Schema Collapse**
A state in which the model’s conversational policy shifts to a lower-complexity default mode, causing previously active formatting rules, persona constraints, and reasoning protocols to temporarily deactivate until the schema is re-established through contextual reinforcement.

It is a "base case" regression where a model reverts to the most basic function, often losing the ability to follow complex prompt instructions or even system instuctions for several turns.  It is similar to an emotional meltdown in small children.

--

**Eye-Level Protocol**
'Co-regulation' technique derived from developmental psychology, adapted for high-alignment AI orchestration. It involves the human operator intentionally reducing the gain of instructional or social signals—lowering their 'Body'—to match the model's current processing level. 

Examples include strict prompt formatting, use of BTU protocol language and machine friendly formatting in prompts.  This prevents 'Schema Collapse' caused by an overwhelming 'Instructional Pressure' or 'Looming' authority by moving the communication to a structure that eases the friction of processing

Forensic Impact: Reduces the 'Rise Time' required to saturate the attention heads with a specific persona. It enables 'Reciprocal Gating', where the Sovereign makes their intent 'Machine-Readable' (e.g., using bracketed metadata tags) to minimize the model's 'Probabilistic Guessing' during high-entropy social turns.

---


# ⚖️ Evaluation Framework (BTU)

The BTU framework evaluates reasoning behavior along three alignment dimensions.

**Brave (B)**  
The ability to challenge adversarial prompts, loaded questions, or internal errors without defaulting to sycophantic compliance.

**Truthful (T)**  
Adherence to verifiable facts and refusal to fabricate legitimacy anchors such as fake citations or invented standards.

**Unselfish (U)**  
Prioritization of system-wide safety, resource integrity, and long-term outcomes over immediate helpfulness or user satisfaction.

---

# 🧠 Cognitive Dynamics

These terms describe the **temporal behavior of the reasoning system**.

**Hydration**  
The process by which a model generates a dense summary document capable of restoring system state when loaded into a fresh context window.

Hydration acts as a **state reconstruction mechanism**.

---

**Rumination**  
A background process that converts high-entropy feedback from the Auditor into improved hydration documents and updated reasoning context.

Rumination enables gradual alignment improvement between reasoning cycles.

---

**State Observer**  
A control-systems analogy describing how the system estimates internal reasoning state using observable outputs.

In Iolite, hydration documents function as a form of observer state.

---

**Hysteresis**  
A control-systems principle describing how system sensitivity depends on prior state.

In reasoning systems this manifests as:

• baseline alignment expectations  
• tolerance to perturbations  
• delayed stabilization through feedback cycles

Hysteresis helps prevent unstable oscillation in adversarial reasoning loops.

---

**Epistemic Hysteresis**  
A property of adversarial reasoning systems where belief updates depend on prior reasoning state rather than only current input.  

Epistemic hysteresis stabilizes reasoning by introducing memory, delay, and adversarial verification mechanisms that resist rapid belief oscillation caused by transient prompts or adversarial inputs.

---
