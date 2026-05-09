# DESIGN.md

## Overview

Skill Organizer web should feel sharp, technical, and deliberate.

The site is not a generic SaaS landing page. It should look like a precise tool for people who already work with agent tooling, CLIs, and structured automation. The visual language should communicate confidence, clarity, and control.

The current direction is a dark, high-contrast interface with cyan, violet, and amber accents. Keep that identity.

## Design Principles

1. Precision over decoration
   Use strong alignment, deliberate spacing, and clear hierarchy. Visual effects should support structure, not distract from it.

2. Technical, not sterile
   The UI should feel engineered and tool-oriented, but still rich. Use glow, gradients, and glass surfaces with restraint.

3. Dense ideas, readable presentation
   This product has a lot of conceptual weight. Layouts should break complexity into well-scanned sections, panels, and terminal-like surfaces.

4. Reuse visual primitives
   Prefer a small set of reusable section, surface, button, chip, and terminal patterns across the site.

5. Motion should clarify state
   Animation is allowed when it explains flow, transition, or emphasis. Avoid decorative movement that does not teach the user something.

## Audience

Primary audience:

- developers using agent tools
- people who understand skills, routing, local tooling, and CLI workflows
- users comparing skill-organizer to manual flat-folder maintenance

The writing and design should assume technical literacy.

## Color System

### Base

- Background: near-black blue
- Panel background: deep blue-black translucent surfaces
- Border: cool low-contrast white/blue borders
- Text primary: near-white
- Text secondary: muted blue-white

### Accents

- Cyan: primary highlight, compatibility, active energy
- Violet: organization, structure, system identity
- Amber: warnings, attention, operational emphasis
- Rose: destructive, duplicate, or error-like states

### Usage Rules

- Use cyan for active and interactive emphasis
- Use violet for structure, product identity, and organizational concepts
- Use amber for CLI, system notices, and warnings
- Use rose only for deletion, duplication, or negative states

## Typography

### Fonts

- Display: `Oxanium`
- Body: `Source Serif 4`
- Code: `JetBrains Mono`

### Rules

- Headings use the display font
- Long-form body copy uses the serif body font
- Commands, paths, labels inside terminals, and technical values use the mono font
- Eyebrows should be uppercase with wide tracking

## Spacing

### Section Rhythm

- Home sections should generally use a shared centered shell with a max width around `84rem`
- Standard top spacing should be large and consistent across sections
- Section internals should prefer `gap`-based layout instead of ad hoc margins

### Density

- Marketing copy can breathe
- Terminal content and docs content can be denser
- Do not over-compress cards or lists at desktop sizes

## Surfaces

### Default Surface

- Rounded corners
- Faint border
- Deep translucent dark background
- Soft inset highlight or subtle outer glow when needed

### Strong Surface

- Used for primary hero panels, terminal shells, featured cards
- Can include layered gradients and stronger shadow

### Rules

- Avoid flat gray boxes
- Surfaces should feel luminous but controlled
- Keep contrast strong enough to separate stacked panels clearly

## Buttons And Chips

### Primary CTA

- Rounded pill
- Bright cyan-violet-amber gradient
- Dark text on bright fill

### Secondary CTA

- Rounded pill
- Transparent or dark fill
- Thin border
- White or muted text with stronger hover state

### Chips / Badges

- Rounded pill
- Small mono or compact body typography depending on context
- Used for modes, states, supported concepts, and tags

## Terminal Style

Terminal-inspired UI is a core visual motif.

### Rules

- Use terminal shells for command output, overlap analysis, and code-driven workflows
- Keep terminal bars compact with traffic-light dots and mono titles
- Terminal interiors should feel structured and readable, not noisy
- Reuse a shared terminal shell component when possible

## Layout Guidelines

### Home

- Home sections should live in `src/views/home/components/`
- Reusable UI belongs in `src/components/`
- Prefer shared section primitives for shell and heading layout

### Docs

- Docs-specific components should live in `src/views/docs/components/`
- Docs layout should emphasize scanability and stable navigation
- Keep docs page width readable; avoid overly wide prose

## Motion

Motion should be minimal and purposeful.

Allowed:

- reveal-on-scroll
- terminal state transitions
- hero transform animations that explain source-to-target flow
- subtle hover scale or border emphasis on actions

Avoid:

- large ornamental motion
- bouncing or playful effects
- animation that competes with technical content

Respect reduced motion preferences.

## Tailwind Strategy

Use Tailwind as the default styling system.

### Prefer

- utility classes in markup
- shared Astro components for repeated visual structures
- a tiny residual CSS layer only for:
  - keyframes
  - prose/content selectors
  - scrollbar styling
  - unavoidable selector-based state helpers

### Avoid

- new component-scoped `<style>` blocks
- one-off CSS classes for simple layout or typography
- duplicating the same section and card styles across views

## Component Boundaries

### Shared Components

Store reusable pieces in `src/components/`.

Examples:

- section shell
- section header
- surface/card primitives
- terminal shell
- buttons
- chips
- shared logos

### View Components

Store view-specific pieces in:

- `src/views/home/components/`
- `src/views/docs/components/`

Interactive or domain-heavy sections can stay view-specific even if they are visually rich.

## Do / Don't

### Do

- build strong hierarchy with shared primitives
- keep spacing and rhythm consistent across sections
- preserve the dark technical visual language
- reuse terminal and surface patterns
- make technical content feel structured and intentional

### Don't

- introduce generic pastel SaaS styling
- overuse gradients in every element
- create shallow components that only rename a `div`
- leave repeated section patterns duplicated across files
- mix Tailwind and bespoke CSS when Tailwind is sufficient
