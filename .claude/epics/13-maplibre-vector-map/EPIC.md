# EPIC 13: MapLibre Vector Map

## Status

In Progress

## Goal

Redesign the web app map from raster OSM tiles to MapLibre GL JS with vector tiles (OpenFreeMap), delivering an F1 telemetry dashboard aesthetic: dark background, muted geography, glowing route polyline, provider selector.

## Context

The current `OsmMapProvider` already uses MapLibre GL JS but with raster tiles from OpenStreetMap. The Yandex providers are the primary fallback chain. The redesign switches the primary provider to `maplibre-vector` using OpenFreeMap vector tiles with a fully custom dark style.

## Problem Analysis

- Raster tiles: fixed styling, no control over colors/layers, large bandwidth
- No way to hide POI, mute minor roads, or apply custom color schemes
- OpenFreeMap: free, no API key, uses OpenMapTiles vector schema, MapLibre-compatible

## Best Practice Research

- OpenFreeMap: `https://tiles.openfreemap.org/planet` (TileJSON), glyphs hosted
- MapLibre GL JS v5 style spec: version 8, vector source type, expression filters
- Dark map palette: background #0a0e14, water #0c1828, roads muted blues

## Solution Design

1. Add `'maplibre-vector'` to `MapProviderType`
2. Create `OpenFreeMapProvider` — same interface as `OsmMapProvider`, custom dark style
3. Update `RouteMap`: `createProvider`, `getInitialProvider`, `getFallbackProvider`
4. Add provider selector overlay inside `RouteMap` component

## Architecture Notes

- `OpenFreeMapProvider` follows identical `MapProvider` interface
- Fallback chain: `maplibre-vector` → `osm`
- Selector: compact overlay in map corner, persisted in `localStorage`

## Tasks

- [x] Add `'maplibre-vector'` to `MapProviderType`
- [x] Create `OpenFreeMapProvider.ts`
- [ ] Update `RouteMap.tsx` (createProvider, getInitialProvider, getFallbackProvider, selector UI)

## Acceptance Criteria

- Dark vector map renders with OpenFreeMap tiles
- Route glow line visible on dark background
- Provider selector works: maplibre-vector / osm / yandex
- Fallback works if OpenFreeMap fails
- No API key required for maplibre-vector

## Test Plan

- Visual: dark map loads, no POI clutter, roads muted
- Route: GPS track with green glow line visible
- Fallback: disconnect from internet → falls back to osm
- Selector: switch between providers works

## Documentation Plan

N/A

## Implementation Log

- 2026-06-13: created OpenFreeMapProvider with custom dark style, added maplibre-vector type

## Final Report

TBD
