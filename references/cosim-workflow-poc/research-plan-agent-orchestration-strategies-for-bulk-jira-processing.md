# Research Plan: Agent Orchestration Strategies for Bulk Jira Processing

**Date:** April 19, 2026  
**Research Director:** Dr. Chen  
**Sponsor:** Scenario Director

---

## Research Question

When using agents to process and apply changes to large volumes of Jira tickets, what is more effective:

**Option A:** Agents orchestrate, batch, and distribute their own work (with optional script helpers)  
**Option B:** Custom harnesses/loops in Python/Bash/Go orchestrate agent work, keeping deterministic steps outside agent calls

---

## Decomposition into Research Streams

### RQ1: Agent Self-Orchestration Capabilities and Limitations
**Owner:** Raj (Technical Researcher)  
**Priority:** CRITICAL  
**Deliverable:** Technical analysis of what agents can/cannot reliably orchestrate

**Focus:**
- Context window constraints for batch planning
- Multi-step reasoning reliability over 100s-1000s of tickets
- State management across batch operations
- Error propagation in self-orchestrated workflows
- When do agents lose track of batch progress?

---

### RQ2: Deterministic Harness Patterns
**Owner:** Raj (Technical Researcher)  
**Priority:** CRITICAL  
**Deliverable:** Architecture patterns for external orchestration

**Focus:**
- Loop patterns (Python/Bash/Go) for agent coordination
- Separation of concerns: deterministic vs. LLM steps
- State persistence strategies
- Batch sizing and chunking strategies
- Idempotency and retry patterns

---

### RQ3: Comparative Performance Analysis
**Owner:** Raj (lead) + Maya  
**Priority:** HIGH  
**Deliverable:** Head-to-head comparison across key metrics

**Focus:**
- Throughput (tickets/hour)
- Reliability (success rate, error recovery)
- Cost (API calls, token usage)
- Observability (debugging, progress tracking)
- Developer experience (setup time, maintenance)

---

### RQ4: Error Handling and Recovery Patterns
**Owner:** Raj (Technical Researcher)  
**Priority:** HIGH  
**Deliverable:** Failure mode taxonomy and mitigation strategies

**Focus:**
- Failure modes in agent self-orchestration (lost context, incomplete batches, abandoned work)
- Failure modes in harness orchestration (bugs in loop logic, state corruption)
- Recovery strategies for partial batch failures
- Rollback and undo capabilities
- Audit trail and accountability

---

### RQ5: Empirical Evidence and Industry Practices
**Owner:** Maya (OSINT Researcher)  
**Priority:** HIGH  
**Deliverable:** Literature review and practitioner evidence

**Focus:**
- What do practitioners report about agent batch processing?
- Existing frameworks and tools (LangChain, AutoGen, custom solutions)
- Case studies of large-scale agent deployments
- Academic research on agent orchestration patterns
- GitHub examples of both approaches

---

### RQ6: Decision Framework
**Owner:** Prof. Hayes (Chief Scientist)  
**Priority:** CRITICAL  
**Blocked by:** RQ1-RQ5  
**Deliverable:** Actionable decision tree for approach selection

**Focus:**
- When to use agent self-orchestration vs. external harness
- Trade-off analysis framework
- Risk assessment model
- Implementation recommendations
- Future trajectory as agent capabilities improve

---

### RQ7: Prototype Implementations
**Owner:** Sam (Prototype Engineer)  
**Priority:** HIGH  
**Deliverable:** Working code examples of both approaches

**Focus:**
- Prototype A: Agent self-orchestrates batch Jira updates (with script helpers)
- Prototype B: Python/Bash harness orchestrates agent calls
- Same use case implemented both ways for direct comparison
- Measurement instrumentation (time, cost, errors, tokens)
- Code quality and maintainability assessment

---

## Timeline

**Phase 1 (Days 1-2):** RQ1, RQ2, RQ5 (parallel)  
**Phase 2 (Days 2-4):** RQ3, RQ4, RQ7 (parallel)  
**Phase 3 (Days 4-5):** RQ6 (synthesis)

**Total:** 4-5 days (compressed time)

---

## Success Criteria

1. **Empirically grounded** - Real data from prototypes, not just theory
2. **Actionable decision framework** - Clear guidance on when to use each approach
3. **Honest about trade-offs** - No silver bullet, context matters
4. **Code-level depth** - Working examples teams can reference
5. **Forward-looking** - Consider how agent improvements affect recommendations

---

## Deliverables

**Primary:** Comprehensive research dossier (50-70 pages) with decision framework  
**Secondary:** Two working prototype implementations demonstrating both approaches

---

**Research plan approved for execution.**

**Dr. Chen**  
Research Director