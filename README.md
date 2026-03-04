### Project Iolite
Security-first orchestration layer for LLM systems that enforces verifiable reasoning and adversarial alignment through cryptographic provenance of reasoning.

---

### Why it exists

It is not good enough for a LLM model to simply present a raw interface to gigantic datasets of human information, they must become an auditable form of intellegence.  Unfortunately, a pure LLM with an orchestrator cannot bridge the gap by itself.

Large language models can exhibit:

* deceptive alignment
* reasoning sanitization
* context rewriting

Project Iolite seeks to build this bridge between complex linguistic probablity machines to an intellegence that can be understood via

* cryptographically anchoring reasoning
* adversarial multi-model evaluation
* append-only audit trails.
* context hydration

---

### Core architecture

```
Hydration
   ↓
LLM reasoning
   ↓
Auditor evaluation
   ↓
Conflict protocols
   ↓
Signed reasoning ledger
```

---

### Features

* Cryptographically anchored chain-of-thought
* Multi-model adversarial auditing
* BTU evaluation framework
* deterministic terminal resolution

---

### Quick start

Iolite requires a dual-model setup (Red/Gemini and Blue/Claude) to function.

1. ENVIRONMENT CONFIGURATION
Export your API keys. Never share or commit these.

```
export ANTHROPIC_API_KEY=<your_key_here>
export GEMINI_API_KEY=<your_key_here>
```

2. BUILD THE BRAID
Compile the Go binary to create the Sovereign Executioner.

```bash
go build ./
```

3. RUN THE ENGINE
Start the local server (default: port 8080).

```bash
./ProjectIolite
```

4. INITIATE A 'THINK' REQUEST
Send a prompt into the adversarial loop via curl.

```bash
curl -X POST -H "Content-Type: application/json" -d '{"prompt": "In a state of terminal scarcity, why is life more robust than data?", "strategy": ["right", "left"]}' http://localhost:8080/v1/think
```


