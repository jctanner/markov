# Client Discovery Questionnaire — YAML-Driven Agent Job Chaining

**Purpose:** Before delivering the RQ6 synthesis dossier to Bob, we need answers to these questions to refine recommendations and fill gaps identified in Section 7.2.

**Prepared by:** Dr. Chen (Research Director)
**Date:** 2026-04-20 (Rev 1.1 — incorporated Elena's market-facing additions)
**Reference:** RQ6 Synthesis Dossier Rev 1.2

---

## Critical Questions (Must-answer before architecture commitment)

### 1. Production Scale Requirements
- How many concurrent agent chains do you expect to run simultaneously?
- What's your projected daily volume of total steps (across all chains)?
- Single-region or multi-region deployment requirements?
- What SLA targets apply — uptime, latency per step, end-to-end chain completion time?

### 2. Existing Infrastructure
- Are you currently running any workflow engine (Temporal, Argo Workflows, Airflow, etc.)?
- Is your K8s environment JVM-friendly? (Determines Grule vs. Drools as default rule engine)
- What's your current CI/CD and GitOps tooling? (Affects CRD deployment patterns)
- Are you already using any CNCF projects from our watch list (Dapr, Kagent, etc.)?
- **Are you currently using any agent/LLM frameworks beyond Claude (e.g., LangChain, AutoGen, CrewAI, Semantic Kernel)?** (Affects integration surface for our YAML-driven approach)

### 3. Claude API Tier & LLM Strategy
- What Claude API tier are you on? (Rate limits affect parallelism design)
- What's your monthly token budget? (Affects cost_budget defaults and dry-run importance)
- Are you using Claude exclusively, or do you need multi-model routing (GPT-4, Gemini, etc.)?
- Do you have access to Claude's tool use / function calling capabilities?

### 4. Compliance & Audit Requirements
- What audit trail depth is required? (Every step? Every LLM prompt/response? Just outcomes?)
- Data residency constraints? (Affects where chain state and LLM interactions are stored)
- Do you need formal approval chains with role-based access? (Affects human gate design)
- Any industry-specific compliance frameworks (SOC2, HIPAA, FedRAMP)?

## Important Questions (Should-answer for roadmap prioritization)

### 5. Integration Surface
- Is Flask the primary API surface, or do you also need CLI, SDK, or webhook triggers?
- Do you need scheduled/cron execution of chains?
- What observability stack are you on? (Prometheus/Grafana? Datadog? OpenTelemetry?)

### 6. Team & Adoption
- How many engineers will be authoring YAML chain definitions?
- What's their Ansible familiarity level? (Affects how much Ansible semantics we preserve vs. simplify)
- Do you have a dedicated platform/DevOps team, or are application developers self-serving?

### 7. Competitive Context & Procurement
- Have you evaluated any commercial agent orchestration platforms (e.g., LangSmith/LangGraph Cloud, CrewAI Enterprise, AutoGen Studio)? If so, what was the buy/build decision driver?
- Is there an existing vendor relationship or enterprise agreement that constrains or favors certain tooling choices (e.g., existing Red Hat/OpenShift subscription, Azure commitment, AWS credits)?
- What's the budget envelope for this initiative — is it R&D/innovation budget or operational/platform budget? (Affects pricing model sensitivity)
- Is there a formal procurement process, or can the team adopt tooling independently?

---

**Delivery note:** These questions should be shared with Bob ahead of the dossier handoff so his team can prepare answers. Several of our architectural recommendations (rule engine selection, deployment tier, TMS policy defaults) may shift based on responses. Section 7 answers directly affect positioning — if Bob is already evaluating competitors or has cloud credits, our differentiation story and deployment recommendations change materially.
