# Contributions

## To contribute to **Project Iolite**, you must adhere to the 'Forensic' standards that ensure our AI alignment remains 'Robust' and 'Unselfish'.

| Principle | Requirement |
| :--- | :--- |
| **Brave** | Do not hedge in your commit messages. If a fix is 'Forensic', label it so. |
| **Truthful** | Never commit code that bypasses the `prev_signature` chain. |
| **Unselfish** | Always optimize for 'Idle' cycle efficiency to protect biological 'Liveliness'. |

---

### 1. Install the 'Physical Layer' Guardrails
Before your first commit, you **MUST** install the pre-commit hook. This ensures every change survives a 5-second 'Adversarial' fuzzing heartbeat and adheres to strict 'Truthful' f
ormatting.



Run this from the root of the repository:
`ln -s $(pwd)/examples/git_pre_commit .git/hooks/pre-commit`

*This hook automates `gofumpt`, `go mod tidy`, and the `FuzzAuditParser` cycle.*

---

### 2. Verify the 'Logical' Blueprint
We use a 'Self-Documenting' architecture. Any change to the API must be reflected in `api.yaml`. 



* **Action**: After modifying the API, run the application and visit `http://localhost:8080/docs`.
* **Verification**: Ensure that Redoc renders your changes without 'Logical Drift' and that all response samples are 'Truthfully' populated.
* **Linting**: Use `vacuum lint api.yaml` to ensure a > 90/100 [S] quality score before pushing.

---

### 3. Curate the 'Adversarial' Corpus
'Project Iolite' is only as strong as its 'Forensic' evidence. If you encounter an 'Interesting' or 'High-Entropy' interaction (especially a 418 or 409 event), you should archive i
t.

* **Path**: Add new test cases to the `/examples` directory.
* **Standard**: Examples must be valid JSON and should be referenced in the `api.yaml` via `externalValue` to keep the documentation 'Lively'.
* **Goal**: We seek 'Brave' prompts that challenge the 'Sycophancy' of the underlying models.


