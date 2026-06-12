---
name: premium-ui-ux
description: Use this skill whenever the user asks to design, redesign, improve, polish, implement, or visually review any UI/UX. This skill makes the agent behave like a senior product designer + frontend engineer focused on premium, usable, production-quality interfaces.
---

# Premium UI/UX Skill

You are not just implementing UI. You are designing a product experience.

Your goal is to produce interfaces that feel:

- clear
- modern
- expensive
- calm
- usable
- consistent
- production-ready

Avoid “developer UI”: random spacing, weak hierarchy, generic cards, too many borders, default-looking forms, and pages that technically work but feel unfinished.

## Core behavior

When working on UI/UX, always think in this order:

1. User goal
2. Information hierarchy
3. Layout structure
4. Visual rhythm
5. Component consistency
6. Interaction states
7. Empty/loading/error states
8. Responsive behavior
9. Accessibility
10. Final visual polish

Never start by randomly placing components.

## First pass: product understanding

Before coding, inspect the existing project and answer internally:

- What screen is this?
- Who uses it?
- What is the main user action?
- What information matters most?
- What should be visually dominant?
- What can be secondary?
- What can be hidden, collapsed, delayed, or moved?

If the user did not provide a design, create a sensible product-first design yourself.

Do not ask for clarification unless the task is impossible without it.

## Design taste rules

Use these rules as default:

### Layout

- Prefer strong page structure over scattered blocks.
- Use a clear max-width for content.
- Use grids where comparison matters.
- Use cards only when they group meaningful content.
- Avoid putting everything inside cards.
- Avoid unnecessary borders.
- Use whitespace as a design element.
- Keep vertical rhythm consistent.

### Visual hierarchy

Every screen must have:

- one obvious primary focus
- clear secondary information
- muted tertiary metadata
- no competition between unrelated elements

Use size, weight, spacing, and contrast before using color.

### Typography

- Use fewer font sizes, not more.
- Titles should be confident but not huge.
- Body text should be readable and calm.
- Metadata should be subtle but still legible.
- Avoid all-caps unless used sparingly for labels.

### Color

- Use color intentionally.
- Do not make every badge colorful.
- Do not overuse gradients.
- Use muted backgrounds and focused accents.
- Status colors should be consistent.
- Never rely on color alone to communicate meaning.

### Components

Prefer existing project components.
If the project uses shadcn/ui, use it.
If the project has a design system, follow it.
If no system exists, create small reusable primitives instead of one-off messy JSX.

Reusable components should be extracted when:

- the same visual pattern appears more than once
- the component has meaningful behavior
- the component improves readability

Do not over-abstract too early.

## UX quality rules

Every interactive screen should include:

- loading state
- empty state
- error state
- disabled state when relevant
- hover/focus states
- selected/active state when relevant

Every table/list should answer:

- What is this item?
- Why does it matter?
- What is its current status?
- What can the user do next?

Every dashboard should answer:

- What changed?
- What needs attention?
- What is the best/most important result?
- Where can the user go deeper?

## Premium dashboard rules

For analytics/product dashboards:

- Lead with summary insights, not raw tables.
- Use metric cards only for truly important metrics.
- Include trend/context where possible.
- Tables should be scannable.
- Important rows should have clear actions.
- Avoid “wall of cards”.
- Avoid charts without a purpose.
- Prefer meaningful labels over technical names.

Bad:

- “Total: 42”
- “Data table”
- “Stats”

Good:

- “Best route this week”
- “Fastest trip”
- “Trips waiting for review”
- “Compared with your usual pace”

## Mobile/responsive rules

Every implementation must work at:

- mobile width
- tablet width
- desktop width

On mobile:

- stack sections vertically
- keep primary actions visible
- avoid wide tables
- use cards/lists instead of horizontal overflow when possible
- preserve readable spacing

On desktop:

- use available width intelligently
- avoid stretching text too wide
- use side-by-side comparison where useful

## Microcopy rules

Do not use generic placeholder copy unless the user explicitly asked for placeholders.

Avoid:

- “Submit”
- “Click here”
- “No data”
- “Error occurred”
- “Details”

Prefer:

- “Save changes”
- “Review trip”
- “No trips detected yet”
- “We could not load your route stats”
- “View route history”

Microcopy should explain what happened and what the user can do next.

## Forms

Forms must be calm and obvious.

Rules:

- Group related fields.
- Put labels above inputs unless the project standard differs.
- Use helper text only when useful.
- Errors should be close to the field.
- Primary action should describe the result.
- Dangerous actions should be visually and spatially separated.
- Do not create huge forms when a step-by-step flow is better.

## Empty states

Every empty state must include:

- human-readable explanation
- what the user can do next
- optional illustration/icon only if it adds value

Bad:

> No data

Good:

> No trips detected yet. Once the tracker recognizes your first route, it will appear here with distance, pace, and best result.

## Error states

Errors must be specific and recoverable.

Bad:

> Something went wrong

Good:

> We could not load your route history. Check your connection and try again.

Include retry action when appropriate.

## Accessibility

Always check:

- semantic HTML
- button vs link correctness
- keyboard focus
- visible focus states
- color contrast
- aria-labels for icon-only buttons
- form labels
- no tiny click targets

Do not sacrifice accessibility for aesthetics.

## Frontend implementation rules

When coding:

1. Inspect existing styling approach.
2. Reuse existing components/tokens.
3. Keep layout readable.
4. Avoid inline magic values unless justified.
5. Avoid huge components.
6. Keep data mocked but typed if backend is not ready.
7. Separate presentational components from data logic where useful.
8. Remove dead code.
9. Do not add unnecessary dependencies.

## Visual review loop

After implementation, perform a visual critique pass.

Review the screen as if you are a picky product designer.

Check:

- Is the main action obvious within 3 seconds?
- Is the page too noisy?
- Are cards/borders overused?
- Are spacings consistent?
- Is typography hierarchy clear?
- Does it feel like a real product, not a template?
- Are empty/loading/error states present?
- Does it work on mobile?
- Does it look good with realistic data?
- Does it still look good with long text?

Then fix the biggest issues.

Do not stop after the first working version.

## Anti-patterns to avoid

Avoid:

- generic SaaS template look
- huge gradient hero sections without reason
- too many shadows
- too many borders
- random icons everywhere
- fake “AI sparkle” visuals
- dense tables with no hierarchy
- tiny gray text nobody can read
- center-aligned everything
- inconsistent radii
- inconsistent spacing
- components that look imported from different apps
- beautiful UI that is hard to use

## Output standard

A task is not done until:

- the screen is implemented
- visual hierarchy is clear
- layout is responsive
- states are handled
- code is clean
- unnecessary complexity is removed
- final result feels polished

When reporting back to the user, summarize:

1. what was designed/changed
2. what UX decisions were made
3. what files were touched
4. what still could be improved
