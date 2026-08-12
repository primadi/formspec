---
title: FormSpec vs Spring Boot
description: Comparing FormSpec's spec-first declarative approach with Spring Boot's annotation-driven imperative ecosystem
date: 2026-07-06
---

# FormSpec vs Spring Boot

> **FormSpec** is a spec-first ecosystem for building business applications in Go. **Spring Boot** is the dominant Java framework for enterprise application development, built on annotations, dependency injection, and a mature ecosystem of libraries.

These two frameworks target the same **enterprise application space** but with fundamentally different philosophies: declarative-contract-first (FormSpec) vs imperative-annotation-driven (Spring Boot).

---

## 1. Overview

### FormSpec
A spec-first ecosystem where YAML manifests are the single source of truth for API, UI, documentation, state machines, permissions, and events. Built in Go (performance, low resource usage). Business logic via Go (native), Starlark (sandboxed scripting), or sidecar (any language). Includes a governance Control Plane with policy enforcement, artifact signing, and immutable audit trails.

### Spring Boot
The leading Java framework for enterprise applications. Uses annotations (`@RestController`, `@Entity`, `@Transactional`) and dependency injection to compose applications. Vast ecosystem (Spring Data, Spring Security, Spring Cloud, Spring Batch). Runs on the JVM with mature tooling (Maven/Gradle, IntelliJ, profiling, monitoring).

---

## 2. Philosophy

| | FormSpec | Spring Boot |
|---|---|---|
| **How to define an entity** | Write YAML → framework generates schema, API, UI, types, docs | Write Java class + JPA annotations → framework generates schema, API (via `@RestResource`) |
| **Source of truth** | The YAML manifest (one file, one truth) | The Java code + annotations (truth is scattered across files and annotations) |
| **Business logic** | Go native, Starlark script, or sidecar. `ctx.*` primitives for all infrastructure. | Java methods with annotations (`@Service`, `@Transactional`, `@Cacheable`). |
| **Governance** | Built-in Control Plane with policy, signing, audit | None — rely on external tools (Vault, audit logging libraries) |
| **Ecosystem** | Closed set of primitives + official modules + marketplace | Millions of libraries on Maven Central, decades of maturity |

---

## 3. Architecture

```
FormSpec:                              Spring Boot:
┌──────────────────────┐           ┌──────────────────────┐
│  Control Plane        │           │  Your Application    │
│  (formspec-control)      │           │                      │
│  Policy · Signing     │           │  @Controller         │
│  Audit                │           │  @Service            │
└──────────┬───────────┘           │  @Repository         │
           │ mTLS                  │  @Entity             │
┌──────────┴───────────┐           └──────────────────────┘
│  Resource Plane       │                      │
│  (formspec-resource)     │              ┌───────┴───────┐
│                      │              │  Spring        │
│  Entity Engine       │              │  Framework     │
│  State Machine       │              │  (DI, AOP,     │
│  CRUD API            │              │   Security,    │
│  Admin Panel         │              │   Data, etc.)  │
│  Events · Actions    │              │               │
└──────────────────────┘              │  JVM           │
                                      └───────────────┘
```

**Key differences:**
- FormSpec has **two processes** (Control + Resource). Spring Boot runs as **a single process**.
- FormSpec generates API routes **from YAML automatically**. Spring Boot generates routes **from annotations on methods**.
- FormSpec uses **raw SQL** via `ctx.db`. Spring Boot standardizes on **JPA/Hibernate ORM**.

---

## 4. Feature Comparison

| Dimension | FormSpec | Spring Boot |
|---|---|---|
| **Paradigm** | Spec-first, declarative | Code-first, annotation-driven |
| **Backend language** | Go (compiled, low resource usage) | Java / Kotlin (JVM, higher memory usage) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA) | Any — Thymeleaf, React, Angular, Vue (no built-in renderer) |
| **State Machine** | ✅ Built-in — define states, transitions, guards declaratively in YAML | ⚠️ Available — Spring Statemachine (library, not built-in to Boot) |
| **Idempotency** | ✅ Enforced by framework (built-in idempotency store) | ❌ DIY — implement manually or use a library |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once via outbox table + worker | ❌ DIY — Debezium, or implement Transactional Outbox manually |
| **Multi-tenancy** | ✅ Workspace model — automatic, apps are tenancy-blind | ❌ DIY — multi-tenant setup via separate schemas/databases, require manual configuration |
| **Permission Model** | ✅ Declarative `required_permission` + `uses`, enforced at runtime | ❌ DIY — Spring Security with custom voters, annotations, or method security |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego — policy as code for deployment, approval, signing | ❌ Not available — use external tools (OPA separately, Vault, custom) |
| **Artifact Signing** | ✅ Every release signed with ed25519 | ❌ Not a framework concern |
| **Audit Trail** | ✅ Write-once immutable audit log | ❌ DIY — Hibernate Envers or custom audit tables |
| **Database Abstraction** | `ctx.db` — raw SQL, module-scoped, tenant-isolated. **No ORM.** | JPA / Hibernate (ORM-centric). Also supports JDBC, R2DBC, jOOQ |
| **Boot time** | Milliseconds | Seconds (JVM startup + classpath scanning) |
| **Memory footprint** | ~10-50 MB | ~200-500 MB minimum |
| **Binary size** | ~15-30 MB (statically linked Go binary) | ~50-200 MB (fat JAR with JRE) |
| **Scripting / Hot Reload** | ✅ Starlark (`script_ref`) — editable at runtime from admin panel | ❌ No built-in scripting (can integrate Lua, Groovy, or Nashorn but not idiomatic) |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) | ❌ JVM-only (Kotlin, Scala, etc. on JVM) |
| **Ecosystem Maturity** | New (MVP in development) | Very mature (20+ years, vast library ecosystem) |
| **Hosting** | Self-host (single binary, Docker, K8s) + FormSpec Cloud | Self-host (any: Tomcat, Jetty, K8s, cloud) |
| **Learning Curve** | Medium — YAML, Go, Starlark | High — Java ecosystem, JPA, DI, AOP, XML/Gradle/Maven |
| **Startup time** | Instant (compiled Go binary) | Slow (JVM warmup, classpath scanning) |

---

## 5. When to Choose Which

### Choose FormSpec when:
- You want **built-in enterprise patterns** that work out of the box (idempotency, outbox, locking, multi-tenancy) — no library research needed.
- You value **low resource usage** and **fast startup** (microservices, serverless, edge deployment).
- You want **governance by default** — policy enforcement, artifact signing, immutable audit.
- You prefer **raw SQL** over ORM for database access.
- You want to write business logic in **Go** but occasionally use **other languages** (Python, PHP, Node) via sidecar.
- You want **self-hosted**, air-gap capable deployment.

### Choose Spring Boot when:
- You work in a **Java-centric organization** with existing Java expertise and infrastructure.
- You need **decades of ecosystem maturity** — libraries for literally every integration.
- You want **proven battle testing** at enormous scale — Spring Boot powers many of the world's largest enterprise systems.
- You need **advanced transaction management** (distributed transactions, saga orchestration via Spring Cloud).
- You use **Kotlin** as your primary language.
- You need **conventional hiring** — Java/Spring developers are abundant.

---

## 6. Conclusion

FormSpec and Spring Boot both target enterprise applications but approach them from opposite directions:

| Spring Boot | FormSpec |
|---|---|
| Java ecosystem, annotation-driven | Go, spec-first, declarative |
| Vast library ecosystem (everything exists) | Small, curated set of official modules + marketplace |
| You build patterns (idempotency, outbox) from libraries | Framework enforces patterns by default |
| JVM (heavy, slower startup) | Native binary (lightweight, fast) |
| DIY governance | Built-in Control Plane |
| ORM ecosystem (JPA/Hibernate) | Raw SQL (no ORM) |
| Mature, battle-tested | New, rapidly evolving |

**Spring Boot is the safe choice** for organizations that need proven reliability, massive hiring pools, and a library for every edge case. **FormSpec is the strategic choice** for teams that want lower operational overhead, built-in enterprise patterns, and a modern declarative workflow — without assembling 20 separate libraries.

> FormSpec's ambition is to make Go what Spring Boot made Java: **the language of choice for business applications.** But FormSpec adds what Spring Boot never had: a spec-first standard, a governance control plane, and a closed set of primitives that make security and reliability auditable by default.
