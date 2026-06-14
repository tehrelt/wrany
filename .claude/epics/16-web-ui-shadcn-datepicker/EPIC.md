# EPIC 16: Web UI — shadcn Datepicker

## Status

In Progress

## Goal

Replace the hand-rolled date/time picker in the web analytics client with the
canonical shadcn calendar + date-picker pattern, sourced via the shadcn registry.

## Context

`apps/web` already depends on `react-day-picker` and `date-fns` and ships a custom
`Calendar` and `DateTimePicker`. The custom calendar diverged from the upstream
shadcn component (no dropdown caption, ad-hoc class names), making future shadcn
updates and visual consistency harder. The pickers are used in
`TrackingFilters` (window start / window end).

## Problem Analysis

- Custom `calendar.tsx` reimplements styling instead of using the shadcn baseline.
- `DateTimePicker` mixes bespoke micro-typography with the popover/calendar.
- Hard to keep in sync with shadcn upgrades.

## Best Practice Research

shadcn registry (`@shadcn/calendar`) installs the canonical
`Calendar`/`CalendarDayButton` (new-york-v4) backed by `react-day-picker`, with
`captionLayout="dropdown"` month/year selectors. The official `date-picker` demo
composes `Button` + `Popover` + `Calendar`; date+time is achieved by pairing the
calendar with a native time `Input`.

## Solution Design

- Install canonical `@shadcn/calendar` (overwrites `calendar.tsx`, updates
  `button.tsx`).
- Rewrite `date-time-picker.tsx` on top of the canonical Calendar using the
  official Button + Popover composition, plus a time `Input`.
- Preserve the existing `value` / `onChange` / `label` prop contract so
  `TrackingFilters` needs no changes.

## Architecture Notes

UI-only change inside `apps/web`. No API or backend impact.

## Tasks

- [x] Pull shadcn calendar via registry MCP.
- [x] Install `@shadcn/calendar`.
- [ ] Rewrite `DateTimePicker` on canonical Calendar.
- [ ] Verify typecheck/tests.

## Acceptance Criteria

- `DateTimePicker` renders the canonical shadcn calendar with dropdown caption.
- `TrackingFilters` compiles unchanged.
- `tsc` passes.

## Test Plan

- `npm run build` (tsc) in `apps/web`.
- Manual: open filters, pick date + time, confirm ISO value propagates.

## Documentation Plan

This EPIC.md.

## Implementation Log

- Installed canonical `@shadcn/calendar` (updated `button.tsx`, `calendar.tsx`).
- Rewrote `date-time-picker.tsx` using Button + Popover + canonical Calendar +
  time Input, keeping the prop contract.

## Final Report

Pending verification.
