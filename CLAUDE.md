## Work with wiki

On start of every session:
1. Read wiki/hot.md - cache of last session
2. If you need more context, use wiki/index.md
3. On finish of session, update wiki/hot.md

Use separate files for every entity: every module, service, pattern, important decision is separate, distinct page. Between pages use links through [[filename]]

## Structure of wiki/
- hot.md - latest decisions, current status of work and what is still not done
- index.md - catalog of all project components
- architecture.md  - project architecture and key decisions 
- conventions.md - code conventions

## Think Before Coding 

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## General Code Style

### Comments

Write simple one-line professional comment that do not contain meta. Do not use emoji, special symbols and symbols that user cannot type easily using keyboard. Only ASCII symbols for comments. Comments describe only current code, why it is as it is. 

### Clean Code

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.