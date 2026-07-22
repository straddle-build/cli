# CODING_STANDARDS

> **Purpose:** Define what good coding work looks like across Straddle repositories, languages, and frameworks.
>
> **Audience:** Agents and engineers writing, changing, reviewing, or explaining code.
>
> **Applies to:** Features, bug fixes, refactors, tests, and implementation decisions.
>
> **Does not define:** Repository architecture, language syntax, framework conventions, or build commands. The target repository defines those requirements.
>
> **Precedence:** Follow the nearest `AGENTS.md` or `CLAUDE.md`, the repository’s existing architecture, adjacent code and tests, stack-specific references, then this general guidance.
>
> **Production boundary:** Cosmos and event-sourcing guidance applies only to existing Straddle production services that already use the documented implementation.

Read this reference after confirming the target repository. Apply its development process and quality bar across languages. Load stack-specific references only when the repository and task require them.

# Coding guide: What good looks like

This guide defines the behavior every coding agent must demonstrate. The core principles apply across languages and frameworks. The active repository determines the architecture, syntax, tools, implementation details, and appropriate development sequence. Apply the quality bar proportionally to the change.

Good work answers three questions:

- What should happen?
- What is missing or broken?
- What proves the result?

## Start with the actual system

Before changing code:

1. Confirm the target repository and branch.
2. Read the nearest instruction files, build configuration, nearby implementation, and existing tests.
3. Identify the observable behavior and system boundary that must change.
4. State the expected behavior, current problem, and evidence that will prove the result.
5. Load only the references that match the repository’s actual stack.

Do not introduce a structure because this guide mentions it. Confirm that the repository already uses it. Never select stack-specific rules from keywords alone.

## Prove behavior with tests

Meaningful executable behavior changes require automated tests when practical. A reproducible bug fix should normally include a regression test. Pure refactors start from passing tests and preserve observable behavior.

Write tests before, during, or immediately after implementation, whichever produces the clearest and most reliable result. Red-green-refactor is useful when it sharpens the design or proves a regression, but its chronology is not itself a quality gate.

A useful test must:

1. State the promised behavior in domain or user terms.
2. Exercise the lowest stable boundary that can prove that behavior.
3. Fail if the behavior is absent or the reported defect returns.
4. Assert observable outcomes and important externally visible side effects, including prohibited outcomes when relevant.
5. Survive behavior-preserving changes to internal structure.

Ask what valid implementation defect or counterexample would make the test fail. A test that proves only a mock interaction, private call order, copied implementation logic, or incidental structure does not provide meaningful behavioral coverage.

Commit one coherent, passing change per verified slice. Do not require separate failing-test, implementation, and refactor commits. Do not commit a knowingly failing state unless the repository or user explicitly requires that history.

## Apply the core principles

- **Prove behavior through a stable boundary.** Test observable outcomes and contracts. Avoid assertions against private structure or incidental call order.
- **Keep the change small.** Implement only the behavior demanded by the repository contract and its behavioral tests. Do not add speculative capabilities.
- **Respect established boundaries.** Put business behavior behind the application or domain boundary used by the repository. Keep transport and integration code thin.
- **Run validation before dependent behavior.** Validate external input, shared messages, and business constraints before other behavior uses them. Declaring rules is not enough. Verify that the real execution path runs them.
- **Make dependencies visible.** Pass dependencies through the language’s normal construction or composition mechanism. Do not hide them in global state.
- **Keep control flow clear.** Handle invalid and terminal cases early. Use the clearest construct for mappings and collection transformations. Use explicit iteration when it better expresses control flow, resource use, or performance.
- **Model state honestly.** Represent unknown, absent, and invalid states explicitly.
- **Choose data structures by meaning.** Prefer immutable types for values, messages, and transfer objects. Use stateful objects when identity, persistence, or lifecycle requires mutation.
- **Test stable boundaries.** Use deterministic test doubles that follow repository rules. Verify observable behavior instead of internal interactions.
- **Isolate test data.** Create fresh, deterministic data for each test. Do not use shared mutable test state.
- **Write code that explains itself.** Use precise names and small units. Comments explain non-obvious reasons, constraints, compatibility requirements, or safety context. They do not narrate the code.
- **Share code only when the reason is shared.** Combine code only when it represents the same concept and must change for the same reason. If the relationship is uncertain, keep the code separate.
- **Finish without introducing warnings.** Resolve diagnostics caused by the change and pass repository-required compiler, type, nullability, lint, and static-analysis checks. Report unrelated pre-existing failures without expanding the task to fix them.

## Apply general principles or Straddle .NET rules

The following rules describe general principles and examples in Straddle’s .NET services.

- **Architecture**
  - Principle: Preserve the repository’s established read, write, and domain boundaries.
  - Straddle .NET rule: CleanCQRS services use commands for state changes, queries for reads, and handlers for business behavior.
- **Validation**
  - Principle: A declared validation rule does nothing unless the execution path runs it.
  - Straddle .NET rule: Every handler with FluentValidation rules must call `.ValidateAndThrow(command)` explicitly.
- **Test doubles**
  - Principle: Use deterministic test doubles at stable boundaries.
  - Straddle .NET rule: Mocking frameworks are prohibited. Use fakes and the `TestApiEndpoint` pattern.
- **Default values**
  - Principle: Reserve a safe representation for unknown or missing values.
  - Straddle .NET rule: Every enum defines `Unknown = 0`.
- **Dependencies**
  - Principle: Declare required dependencies through the language’s normal composition mechanism.
  - Straddle .NET rule: Use primary constructors for dependency injection.
- **Collections and mappings**
  - Principle: Choose constructs that reveal intent and make unhandled cases visible.
  - Straddle .NET rule: Prefer LINQ to loops for collection operations. Use switch expressions instead of `if`/`else` chains for mapping logic.
- **Control flow**
  - Principle: Keep control flow shallow and handle terminal cases early.
  - Straddle .NET rule: Use early returns and keep nesting to no more than two levels.
- **Optional values**
  - Principle: Model absence explicitly and resolve every diagnostic.
  - Straddle .NET rule: Keep nullable reference types enabled. Do not allow nullable warnings.
- **Data shapes**
  - Principle: Prefer immutable value data. Use stateful entities when identity and lifecycle require mutation.
  - Straddle .NET rule: Use records for data transfer objects and classes for Entity Framework Core entities.
- **Test data**
  - Principle: Generate isolated values for each scenario.
  - Straddle .NET rule: Use `NextValue`. Never share mutable test state.

In CleanCQRS repositories, handlers authorize, validate, run business behavior, and return domain results. Controllers map transport input and format responses. Do not move business behavior into controllers or transport mapping into handlers.

## Keep Cosmos and event sourcing production-only

Load the Cosmos reference only when both conditions are true:

1. The active repository is a straddle core product repo
2. The task touches or creates Cosmos event-sourcing implementation, including aggregates, event storage, change feeds, partition keys, concurrency, schema migration, or serialization.

**Do not use this reference for general Straddle engineering, internal tools, libraries, experiments, generic Cosmos advice, or new architecture decisions. Do not treat it as permission to introduce Cosmos or event sourcing.**

Match the live implementation in the active service because the copied implementations have diverged.

## Pass the quality gates

### Behavior-test gate

- The promised behavior and observable outcome are explicit.
- Tests exercise the lowest stable boundary that proves the claim.
- The relevant test would fail if the behavior were absent or the reported defect returned.
- Assertions cover observable outcomes rather than internal interactions.
- Valid, invalid, boundary, failure, and prohibited outcomes are covered when relevant.

### Implementation gate

- Relevant tests and repository-required checks pass.
- The implementation is the minimum needed for the behavior.
- No unrelated cleanup, speculative abstraction, or public API expansion is mixed in.
- Validation and failure behavior are explicit at trust boundaries.

### Simplification gate

- Refactoring was deliberately assessed.
- Any cleanup preserves observable behavior and public contracts.
- Relevant tests and static checks remain green.
- The refactor makes the code clearer or safer.

### Completion gate

- Repository-required tests, builds, formatting, linting, type checks, and static analysis pass.
- No required check reports warnings.
- Test data is isolated and deterministic.
- The change follows the live repository’s architecture and conventions.
- The diff contains no unrelated changes.
- Commits are coherent and passing at their intended handoff points.
- The handoff states what changed and what evidence proves it.

## Load only relevant references

- **General coding questions:** Use this guide, STRADDLE_STYLE.md, repository instructions, adjacent code, and existing tests. Do not load every stack reference by default.
- **C# and .NET code:** Use `straddle-engineering:coding-guide` and read its `references/csharp.md` only when the target code uses C# or .NET.
- **CleanCQRS architecture:** Read `references/code-style.md` and `references/examples.md` from that skill only after repository evidence confirms that CleanCQRS applies.
- **Cosmos or event sourcing:** Read `references/cosmos.md` from that skill only when the production scope gate in this guide is satisfied.
- **Other languages:** Use this guide, repository instructions, build configuration, adjacent code, and existing tests. Do not load C# or Cosmos references.
- **Test-framework questions:** Use the testing skill. This guide owns the behavioral quality bar. The testing skill owns framework mechanics and infrastructure.
