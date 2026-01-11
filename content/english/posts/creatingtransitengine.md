---
date: '2026-01-10T20:01:00-03:00'
draft: false
title: 'Building PathCraft: An Open-Source Routing Engine in Go'
tags:
  - algorithm
  - projects
---

## Introduction

When I started building **PathCraft**, I had a simple goal: create a routing engine that I could actually understand, extend, and deploy without vendor lock-in. What began as an experimental side project has evolved into a modular, multimodal routing engine that handles everything from pedestrian navigation to public transit routing.

In this post, I'll share the journey of building PathCraft. Architectural decisions, the algorithms powering it, and the lessons learned along the way.

---

## What is PathCraft?

PathCraft is an experimental walking and transit routing engine written in Go. It's designed to be:

- A **reusable Go library (SDK)** that developers can embed in their applications
- A **CLI application** for quick routing queries
- An **HTTP server** for production deployments
- An **embeddable engine** for integration into larger systems

The core philosophy? **Correctness first, performance second.** Too many routing engines sacrifice readability and maintainability for marginal performance gains. PathCraft takes a different approach.

---

## The Tech Stack

### Why Go?

Go was a natural choice for this project:

1. **Performance**: Go's compiled nature gives us near-native performance without sacrificing developer productivity
2. **Simplicity**: The language's minimalist design aligns with PathCraft's philosophy of clarity over cleverness
3. **Concurrency**: Built-in goroutines and channels make parallel routing a future possibility
4. **Single Binary**: Easy deployment—just ship a binary, no runtime dependencies

### Core Components

- **Custom A\* Implementation**: Hand-rolled for maximum control and educational value
- **RAPTOR Algorithm**: For efficient public transit routing
- **OSM Parser**: Ingests OpenStreetMap data to build walkable graphs
- **GTFS Parser**: Handles General Transit Feed Specification data for transit schedules
- **Haversine Calculations**: Geographic distance computations for heuristics

---

## Architecture: The Modular Monolith

One of the most important decisions was the architecture. I initially considered a Hexagonal (Ports and Adapters) architecture, but ultimately chose a **Modular Monolith** approach.

### Why Not Hexagonal?

For a project focused on algorithmic correctness and rapid iteration, Hexagonal architecture introduces unnecessary overhead. The indirection layers and abstraction boundaries, while valuable for large enterprise systems, would slow down development without providing proportional benefits.

### The Modular Monolith Advantage

The codebase is organized into well-defined modules with explicit boundaries:

```
/internal         → Private core logic
  /graph          → Graph data structures
  /geo            → Geographic calculations
  /routing        → A*, RAPTOR algorithms
  /osm            → OpenStreetMap parsing
  /gtfs           → Transit data parsing

/pkg/pathcraft/engine  → Public API (the only thing users import)
/cmd/pathcraft         → CLI entrypoint
/web                   → Visualization tools
```

This structure maintains low coupling and high cohesion while keeping the system easy to reason about. The dependency flow is strict: **core logic never depends on infrastructure** (HTTP, CLI, file I/O).

---

## Deep Dive: The A\* Algorithm

At the heart of PathCraft's walking router is the A\* algorithm. Here's a simplified view of how it works:

### The Core Idea

A\* combines the best of Dijkstra's algorithm (guaranteed shortest path) with heuristic-guided search (faster exploration). For each node, we calculate:

```
f(n) = g(n) + h(n)
```

Where:
- **g(n)**: Actual cost from start to node n
- **h(n)**: Estimated cost from n to the goal (our heuristic)

### The Heuristic: Haversine Distance

For geographic routing, we use the Haversine formula to calculate the great-circle distance between two points on Earth. This gives us an admissible heuristic—it never overestimates the actual distance, guaranteeing we find the optimal path.

### Priority Queue Implementation

The algorithm maintains an open set as a priority queue, always exploring the most promising node first. Go's `container/heap` package provides the foundation, with a custom `pqItem` struct tracking node IDs and priorities.

---

## Deep Dive: RAPTOR for Transit Routing

For public transit, we implemented the **RAPTOR** (Round-bAsed Public Transit Optimized Router) algorithm. Unlike traditional graph-based approaches, RAPTOR works directly on timetable data.

### How RAPTOR Works

1. **Rounds**: Each round represents one additional transit leg (transfer)
2. **Route Scanning**: For each marked stop, scan all routes serving that stop
3. **Transfer Processing**: After route scanning, process footpath transfers
4. **Pareto Optimality**: Track both arrival time and number of transfers

### Why RAPTOR?

- **Speed**: Typically faster than graph-based approaches for transit
- **Simplicity**: The algorithm is elegant and easy to understand
- **Flexibility**: Easy to add constraints (max transfers, walking distance)

---

## The Engine Facade

The `Engine` struct in `/pkg/pathcraft/engine` serves as the public API—the only package external users should import.

```go
type Engine struct {
    graph     *graph.Graph
    gtfsIndex *gtfs.StopTimeIndex
}

// Core methods
func (e *Engine) Route(req RouteRequest) (RouteResult, error)
func (e *Engine) LoadOSM(path string) error
func (e *Engine) LoadGTFS(stopTimesPath, tripsPath string) error
```

The engine **orchestrates**, it doesn't compute. This separation keeps HTTP/CLI concerns from leaking into algorithms.

---

## Handling GTFS Time: The >24:00:00 Problem

One interesting challenge was handling GTFS time formats. In GTFS, times can exceed 24:00:00 to represent service that runs past midnight. A trip departing at `25:30:00` means 1:30 AM the next day.

We built a custom `time` package in `/internal/time` that handles this edge case transparently, keeping the rest of the codebase clean.

---

## Development Principles

Throughout development, I've adhered to several principles:

### 1. Pure and Deterministic Core

The routing algorithms are pure functions. Given the same graph and query, they always produce the same result. No hidden state, no randomness, no side effects.

### 2. Testability First

Every routing algorithm has comprehensive tests. Edge cases are mandatory. This isn't just good practice—it's essential when you're implementing algorithms where bugs can be subtle and hard to detect.

### 3. No Premature Abstraction

I resisted the urge to abstract too early. Interfaces are introduced only when there's a concrete need, not based on hypothetical future requirements.

### 4. Comments Explain Why, Not What

The code should be self-documenting for *what* it does. Comments are reserved for explaining *why* certain decisions were made.

---

## Current Status & Roadmap

### Completed

- OSM parsing and graph construction
- A\* routing for walking
- CLI interface
- GeoJSON export
- HTTP server mode with `/route` and `/health` endpoints
- GTFS ingestion
- RAPTOR algorithm implementation
- Basic map visualization

### In Progress

- Walk + Transit integration
- Time-dependent routing
- Performance benchmarking

### Future Plans

- Graph contraction for faster queries
- Caching strategies
- WASM/JavaScript bindings
- gRPC API
- Plugin system

---

## Lessons Learned

### 1. Start Simple, Iterate Fast

The first version of PathCraft was embarrassingly simple. That was intentional. Get something working, then improve it.

### 2. Invest in Tooling Early

A good Makefile, consistent formatting, and automated testing pay dividends. Every hour spent on tooling saves days of debugging later.

### 3. Documentation as Design

Writing documentation forces you to think through your design. If you can't explain it simply, you probably don't understand it well enough.

### 4. Algorithms Are the Easy Part

The A\* and RAPTOR implementations were straightforward. The hard parts? Data parsing, edge cases, and making everything work together seamlessly.

---

## Try It Yourself

PathCraft is open source and available on GitHub. To get started:

```bash
# Clone the repository
git clone https://github.com/danielscoffee/pathcraft

# Build the CLI
make build

# See available commands
./bin/pathcraft --help
```

You'll need an OSM file for your area of interest. The `examples/` directory contains sample data to get you started.

---

## Conclusion

Building PathCraft has been an incredible learning experience. From implementing classic algorithms to designing clean interfaces, every challenge has taught me something new.

The goal remains the same as day one:

> "The open-source routing engine you deploy when you don't want vendor lock-in."

If you're interested in routing algorithms, Go development, or just want to understand how navigation apps work under the hood, I hope PathCraft can serve as both a useful tool and an educational resource.

Contributions are welcome. Happy routing!

## Further Reading

- [A\* Search Algorithm - Wikipedia](https://en.wikipedia.org/wiki/A*_search_algorithm)
- [RAPTOR Paper - Microsoft Research](https://www.microsoft.com/en-us/research/wp-content/uploads/2012/01/raptor_alenex.pdf)
- [OpenStreetMap](https://www.openstreetmap.org/)
- [GTFS Specification](https://gtfs.org/)

