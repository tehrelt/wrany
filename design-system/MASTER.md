# WR any% Design System

## Product Positioning

WR any% is a personal movement intelligence product. It automatically turns
everyday travel into comparable routes, attempts, and best laps. The interface
must emphasize passive capture, trustworthy telemetry, and measurable progress.
Never present manual start or finish as a primary action.

## Visual Style

- Dark cockpit telemetry command center.
- Technical grid meets motorsport pit wall.
- Asymmetric, dense, and highly legible.
- Near-black workspace with technical grid lines.
- Acid green marks healthy tracking and personal records.
- Cyan supports geospatial and informational data.
- Amber marks confidence warnings and pending states.
- Red is reserved for errors and destructive actions.
- Use thin borders, restrained elevation, and tabular figures.
- Avoid decorative gradients, oversized glass effects, and generic card grids.

## Palette

| Token | Value | Usage |
|---|---:|---|
| Canvas | `oklch(0.115 0.018 255)` | Application background |
| Surface | `oklch(0.155 0.022 252)` | Cards and panels |
| Ink | `oklch(0.94 0.012 235)` | Primary text |
| Muted ink | `oklch(0.66 0.025 235)` | Secondary text |
| Graphite | `oklch(0.09 0.018 255)` | Navigation |
| Signal green | `oklch(0.82 0.19 135)` | Primary and healthy |
| Cyan | `oklch(0.64 0.13 225)` | Maps and informational |
| Amber | `oklch(0.72 0.15 75)` | Pending and caution |
| Red | `oklch(0.58 0.22 27)` | Error and destructive |
| Border | `oklch(0.885 0.015 100)` | Dividers and controls |

All text and status combinations must meet WCAG AA contrast.

## Typography

- Font: Geist Variable, already bundled.
- Display: 32/36, weight 650, tracking `-0.035em`.
- Page title: 24/30, weight 650, tracking `-0.025em`.
- Section title: 16/24, weight 600.
- Body: 14/22, weight 400.
- Label: 12/16, weight 600.
- Metadata: 12/16, weight 400.
- Never use body text below 12px.
- Use `tabular-nums` for time, distance, speed, and deltas.

## Spacing Scale

Use a 4px base: 4, 8, 12, 16, 20, 24, 32, 40, 48.
Panel padding is 16px mobile and 20-24px desktop. Page sections use 24-32px.

## Radius Rules

- Controls: 10px.
- Small badges: 999px.
- Cards: 3px.
- Major panels: 4px.
- Application shell: 20px.
- Maps follow their containing panel radius.

## Shadows And Elevation

- Level 0: border only.
- Level 1: `0 1px 2px` with 5% graphite.
- Level 2: `0 16px 40px` with 8% graphite.
- Navigation may use Level 2.
- Hover must not translate cards or cause layout shift.

## Card Rules

- Every card needs a clear information purpose.
- Prefer label, primary value, context, then action.
- Metric cards use one icon and one comparison.
- Selected cards use border, background, and text change.
- Do not rely on color alone.
- Interactive cards use buttons or links, not clickable divs.

## Table Rules

- Use semantic `table`, `thead`, `th`, and `tbody`.
- Headers remain readable at 12px minimum.
- Align numeric columns right and use tabular figures.
- Preserve a text marker for best attempts.
- On mobile, allow contained horizontal scrolling.
- Keep row height at least 44px for interactive rows.

## Chart Rules

- Favor lines for trends and horizontal bars for comparisons.
- Attempt charts use best result as a visible baseline.
- Always show numeric values near compact charts.
- Do not use radar charts for primary decisions.
- Use no more than five series.
- Charts need labels, accessible summaries, and empty states.

## Map UI Rules

- Maps are analytical panels, not decorative backgrounds.
- Show provider and health unobtrusively.
- Start and finish markers require text or legend distinction.
- Selected points need a high-contrast halo.
- Map controls need 44px touch targets.
- Loading preserves final map height.
- Errors provide retry or provider fallback context.

## Loading States

- Use skeletons matching final geometry.
- Keep stable panel heights.
- Announce long loading with `aria-busy`.
- Avoid full-page spinners.

## Empty States

- State what is missing.
- Explain automatic tracking behavior.
- Offer a relevant next step without adding start or finish actions.
- Use one quiet icon and concise copy.

## Error States

- Use `role="alert"`.
- Explain failed area and recovery.
- Preserve unaffected partial data.
- Keep destructive colors local to error content.

## Focus States

- Every interactive element needs a visible 2px ring.
- Use emerald ring with 2px canvas offset.
- Never remove outlines without replacement.
- Keyboard order follows visual order.

## Responsive Behavior

- Desktop: persistent navigation, list-detail workspace.
- Tablet: compact navigation and balanced two-column layouts.
- Mobile: top navigation, stacked panels, full-width actions.
- Maps use 320px minimum height.
- Tables scroll inside their panel.
- Test widths: 375, 768, 1024, and 1440px.
- No page-level horizontal scrolling.

## Accessibility Rules

- Meet WCAG 2.2 AA.
- Use landmarks and descriptive headings.
- Buttons require accessible names.
- Icons supplement visible labels.
- Status uses text plus color or icon.
- Tables include scoped headers.
- Errors use live announcements.
- Respect `prefers-reduced-motion`.
- Touch targets are at least 44px where practical.

## Component Usage

- `MetricCard`: one measurable value.
- `RouteCard`: route identity and attempt summary.
- `TripCard`: detected trip summary and status.
- `StatusBadge`: lifecycle or system state.
- `ConfidenceBadge`: text confidence and icon.
- `EmptyState`: missing data guidance.
- `ErrorState`: recoverable query failure.
- `LoadingSkeleton`: stable async placeholder.
- `PageHeader`: title, description, contextual actions.
- `SectionHeader`: section title and optional metadata.
- `AttemptsTable`: ranked route attempts.
- `RouteMapPanel`: map, legend, and provider state.
- `ComparisonDelta`: signed difference versus baseline.
- `ActivityTypeBadge`: movement classification.

Prefer composition. Keep API queries in page containers. Keep display components
free from fetching logic. Temporary demo data must be isolated and labeled.
