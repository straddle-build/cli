# STRADDLE_STYLE

## Purpose

Style is design. It affects how software behaves, how safely it changes, and how easily people understand it.

Our design priorities are:

1. Safety
2. Performance
3. Developer experience

The ability to change behavior quickly and safely is the highest form of code quality. Every principle in this guide supports that goal.

This guide defines engineering principles that apply across languages, frameworks, and products. It does not define language syntax, formatting rules, repository architecture, or hardware-specific optimization techniques.

## Precedence

Apply guidance in the following order:

1. The nearest repository instructions
2. The system’s existing contracts and architecture
3. Established patterns in adjacent code and tests
4. This guide
5. Personal preference

Do not import a structure from another repository because it is familiar. Match the actual problem and the system you are changing.

When principles conflict, protect safety first, then performance, then developer experience. Look for a simpler design that advances all three before accepting a tradeoff.

## Think before implementation

Build a clear mental model before changing the system.

Before implementation:

- State what should happen.
- State what is missing or broken.
- Identify the boundary that owns the behavior.
- Identify failure modes and invalid states.
- Define what evidence will prove the result.
- Estimate scale and resource use when they could affect the design.
- Read the existing implementation and tests.

Spend design effort while the code is still inexpensive to change. A problem found during design costs less than the same problem found in production.

Do not hide uncertainty. Name assumptions, unknowns, and tradeoffs before they become implementation decisions.

## Earn simplicity

Simplicity is not the first solution that comes to mind. It is the result of understanding the problem well enough to remove what does not belong.

Prefer the smallest design that fully satisfies the requirements:

- Do not add unrequested capabilities.
- Do not add abstractions for one-time operations.
- Do not add configuration for hypothetical needs.
- Do not introduce distributed components when local composition works.
- Do not combine code solely because it looks similar.
- Do not make a solution simpler by ignoring required behavior.

Create an abstraction only when multiple uses represent the same concept and must change for the same reason. Similar syntax does not prove shared meaning. When the relationship is uncertain, keep the code separate.

## Keep changes coherent

Apply design-smell checks while planning, writing, and reviewing every change. The goal is to prevent unclear ownership, accidental duplication, speculative abstractions, and unnecessary indirection from entering the codebase.

As the change develops:

- Give every new concept an honest, specific name.
- Put behavior with the domain concept and data it owns.
- Keep values that form one concept together.
- Represent important domain concepts explicitly.
- Centralize decisions that must remain consistent.
- Keep related changes close together.
- Separate code that changes for unrelated reasons.
- Add abstractions only for requirements that exist.
- Make every layer, delegation, dependency, and inheritance relationship earn its place.
- Prefer the smallest design that remains clear and complete.

Treat a smell as a prompt to investigate, not proof that the code is wrong. Name the concrete cost before changing the design. If the proposed fix introduces more coupling, indirection, or speculative structure than it removes, keep the simpler design.

For meaningful executable-code changes, inspect the complete change with `code-review-and-quality`, resolve accepted findings, then run `code-simplification`. Review and simplification should be read-only when delegated; the implementer owns every accepted edit and final verification. `NO_CHANGE` is a valid simplification result. Small documentation, metadata, configuration, or mechanical changes need only focused self-review. Never expand either pass into unrelated cleanup.

## Optimize for change

A good design makes the next change easier to understand, implement, verify, and reverse.

Prefer:

- Explicit dependencies over hidden coupling
- Small contracts over broad interfaces
- Reversible decisions over premature commitments
- Clear ownership over shared responsibility
- Local reasoning over behavior spread across the system
- Stable boundaries over internal implementation exposure
- Focused changes over broad rewrites

Complexity must earn its place. If a design makes future changes harder without protecting a real requirement, simplify it.

## Be explicit

Make important behavior visible to readers, tools, and tests.

- Pass required options explicitly when defaults could hide important behavior.
- Represent unknown, absent, and invalid states directly.
- Make limits, timeouts, retry policies, and failure behavior visible.
- Keep dependencies visible at construction or composition boundaries.
- Prefer direct control flow over hidden framework behavior.
- Use names that reveal domain intent.
- Explain why a surprising decision exists.

Do not rely on a default when changing that default could create incorrect or unsafe behavior.

## Build for safety

### Keep control flow understandable

Use clear, direct control flow. A reader should be able to identify the possible paths without mentally executing the entire system.

- Handle invalid and terminal cases early.
- Keep nesting shallow.
- Break complex conditions into named or separately testable decisions.
- Prefer positive conditions when they are easier to verify.
- Make both expected and unexpected branches visible.
- Avoid control flow that depends on hidden side effects.

Do not impose universal bans on recursion, asynchronous work, or other language features. Use them only when they make behavior easier to reason about and the repository supports them.

### Bound work and resources

Unbounded work eventually becomes a reliability problem.

Define limits for operations such as:

- Collection and queue sizes
- Batch sizes
- Retries
- Timeouts
- Pagination
- Concurrency
- Work performed per request or event
- Data retained in memory or storage

When work must remain open-ended, make the reason explicit and add controls that prevent one operation from consuming the system.

### Separate invariants from operating errors

Programmer errors and operating errors require different responses.

An invariant describes a condition that must remain true if the code is correct. Detect invariant violations immediately. Do not continue with state that the system considers impossible.

Operating errors are expected conditions such as invalid input, unavailable dependencies, timeouts, conflicts, or rejected requests. Handle them deliberately and return enough information for the caller or operator to respond.

### Validate at boundaries

Validate data before other behavior depends on it.

Apply validation at:

- External inputs
- Service and module boundaries
- Shared messages and events
- Persistence boundaries
- Business-rule transitions

Declaring a validation rule is not enough. Verify that the real execution path runs it.

Check both spaces:

- The positive space containing values and states the system accepts
- The negative space containing values and states the system must reject

Many failures occur where data crosses between those spaces.

### Handle and test errors

Every error path must have an intentional outcome.

- Do not swallow errors.
- Do not convert specific failures into vague generic responses.
- Test expected operating errors.
- Test invalid input and boundary values.
- Test transitions from valid to invalid states.
- Include failure behavior in the system’s observable contract.

Treat warnings from required compilers, type checkers, linters, and static-analysis tools as failures unless the repository documents a specific exception.

### Keep state local

Minimize the amount of state a reader must track.

- Declare values close to where they are used.
- Use the smallest practical scope.
- Avoid shared mutable state.
- Keep checks close to the operations that depend on them.
- Prefer pure calculations when mutation is unnecessary.
- Centralize state changes when several operations affect the same state.

Distance in time or code increases the chance that a checked condition no longer holds when the system uses it.

## Design for performance

Performance begins with architecture, not micro-optimization. The largest gains usually come from changing the shape of the work before implementation.

For most application code, hardware-specific optimization is not the default. Investigate performance when scale, latency, reliability, or cost can materially affect the design.

### Estimate before implementation

Create rough estimates for relevant resources:

- Network
- Storage
- Memory
- Compute

For each resource, consider:

- Latency
- Throughput
- Frequency of use
- Expected and maximum volume
- Growth over time

The estimate does not need perfect precision. It must be accurate enough to expose a design that cannot meet its requirements.

### Make work predictable

Prefer systems with visible and bounded resource use.

- Batch repeated work when batching reduces overhead without violating latency requirements.
- Avoid unnecessary network and storage operations.
- Prevent one request, event, or customer from monopolizing shared resources.
- Separate coordination work from high-volume processing when the distinction improves reliability.
- Document important capacity assumptions and limits.

Use measurement to validate a design and find local bottlenecks. Do not wait for profiling to discover that the overall architecture cannot meet its requirements.

Do not optimize code that lacks a demonstrated performance need. Clarity remains the default unless evidence justifies additional complexity.

## Improve developer experience

Developer experience is the ease with which someone can understand, use, change, and operate the system.

### Name things precisely

Good names create a clear mental model.

- Use domain language instead of implementation jargon.
- Choose nouns that describe what a thing is.
- Choose verbs that describe what an operation does.
- Include units or qualifiers when values could be confused.
- Use one term consistently for one concept.
- Rename code when the current name hides its responsibility.

A naming problem often reveals a design problem. If a thing is difficult to name, reconsider what it owns.

### Keep interfaces small

Every input, output, and possible state increases what a caller must understand.

- Require only the information an operation needs.
- Return the simplest result that represents the outcome.
- Avoid optional values when a clearer contract is possible.
- Avoid flags that create several unrelated behaviors in one operation.
- Keep failure modes specific and documented.
- Do not expose internal structure without a contract-level reason.

Reduce the number of branches every caller must handle.

### Make code explain itself

Use structure and naming to explain what the code does. Use comments to explain:

- Why a surprising decision exists
- Which external constraint requires it
- Which invariant it protects
- Why an apparently simpler approach is incorrect
- What operational or compatibility risk must remain visible

Do not use comments to narrate obvious code. Do not remove necessary rationale in pursuit of comment-free code.

### Keep code locally understandable

A reader should not need to jump through several files to understand one operation.

- Keep related checks, calculations, and state changes close together.
- Split functions when the split creates a meaningful responsibility.
- Do not extract single-use helpers that make the control flow harder to follow.
- Keep high-level decisions visible in the calling operation.
- Move detailed calculations into focused helpers when that improves clarity.

Use comprehension as the constraint, not an arbitrary line count.

### Make numeric intent explicit

Indexes, counts, sizes, limits, and units may share the same underlying representation but mean different things.

- Name the meaning and unit.
- Make conversions visible.
- Test zero, one, maximum, and boundary values.
- State the intended rounding behavior.
- Avoid implicit conversions that hide loss of precision or changes in meaning.

## Preserve architectural boundaries

Split systems according to stable domain ownership.

- Each domain owns its behavior and data.
- Boundary contracts and versioning are first-class concerns.
- Boundaries map and translate. They do not become containers for business logic.
- Services communicate through supported contracts, not direct access to another service’s storage.
- Dependencies point toward stable domain behavior.
- Infrastructure implements domain-facing contracts.

Do not create a service boundary merely to separate code. When uncertain, prefer the simpler topology. A well-structured monolith is easier to split than tightly coupled services are to recombine.

Apply design principles flexibly. Single responsibility matters, but named principles do not justify unnecessary layers or abstractions.

## Test behavior

Tests prove what the system does. They should survive changes to internal structure.

- State the promised behavior in domain or user terms.
- Test observable outcomes and stable contracts.
- Avoid assertions against private methods or incidental call order.
- Use the smallest test layer that proves the behavior.
- Do not duplicate the same confidence at several layers without a reason.
- Use isolated, deterministic test data.
- Test valid, invalid, boundary, and failure scenarios.
- Test important prohibited outcomes when their absence is part of the contract.
- Treat tests as executable documentation.

Prefer unit tests for isolated behavior, integration tests for collaboration between real components, and end-to-end tests for important cross-system flows. Choose the layer based on the claim being proved, not a fixed quota.

A harmless refactor should not break behavior tests. If it does, the tests may depend too heavily on implementation details.

For each important claim, ask what valid implementation defect or counterexample would make the test fail. A test is not useful merely because the current implementation passes it. It must fail when the promised behavior is removed or the reported defect returns. Tests that only verify mocks, private call order, copied production logic, or incidental structure do not prove the behavior.

## Ship small, accurate changes

Velocity comes from accurate changes that can ship safely, not from rushing.

- Keep changes small enough to review and revert.
- Avoid long-running branches.
- Split unrelated behavior into separate changes.
- Automate repeatable verification and release steps.
- Use staged delivery when it reduces risk.
- Under pressure, narrow scope before lowering the quality bar.

A change is complete when:

- The intended behavior works.
- Relevant tests prove it.
- Required checks pass.
- Known in-scope correctness problems are resolved.
- Operational behavior is understood.
- The change can be reviewed and reversed cleanly.

Do not create a generic technical-debt ticket to excuse unfinished work. Finish the work while the context is fresh, or name the specific dependency or blocker that prevents completion.

New information may justify later changes. That is normal product development, not evidence that the original work was incomplete.

## Prefer proven technology

Use technology that solves the actual problem with the least operational and conceptual burden.

Adopt new technology when it provides a demonstrated advantage that outweighs:

- Migration cost
- Operational risk
- Training cost
- Additional failure modes
- Long-term maintenance
- Reduced reversibility

Familiar and reliable technology is often the better choice. Novelty is not a design goal.

## Review checklist

Before considering a change complete, answer the following questions:

### Behavior

- What happens?
- What was missing or broken?
- What proves the result?

### Safety

- Are inputs validated at the correct boundaries?
- Are work and resource use bounded?
- Are invariants explicit?
- Are expected errors handled and tested?
- Are invalid and boundary states covered?

### Performance

- Could scale, latency, throughput, or cost affect the design?
- Are important resource assumptions documented?
- Is repeated work batched or reduced where appropriate?
- Does any operation perform unbounded work?

### Developer experience

- Do names express domain intent?
- Are interfaces smaller than they need to be?
- Is important behavior explicit?
- Can the change be understood without unnecessary navigation?
- Do comments explain reasons rather than narrate code?

### Changeability

- Does the change match the repository’s architecture?
- Is the scope focused?
- Are dependencies and boundaries clear?
- Are abstractions supported by shared meaning?
- Can the change be reversed cleanly?

### Verification

- Do tests prove behavior rather than implementation?
- Do required checks pass without warnings?
- Does the handoff state what changed and what evidence proves it?

The guide itself may change as Straddle learns. The commitment to safe, fast change does not.

:::
