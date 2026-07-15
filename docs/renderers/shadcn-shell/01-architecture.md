# Arsitektur shadcn-shell

**Updated:** 2026-07-15 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap dari kode `web/`.

## 1. Interpreter Runtime
SPA generik yang di-deploy sekali dan me-render App apa pun dari spec — bukan
build artifact per-app.

## 2. Struktur Kode
`kinds/` (renderer per kind), `engine/` (registry, derive, lifecycle,
permissions), `shell/`, `lib/api/`, `stores/`, `widgets/`, `hooks/`.

## 3. Bootstrap & App Renderer
Bagaimana shell mewujudkan asumsi bootstrap tiap App kind (sidebar-nav, topnav,
landing-page); routing; auth wiring.

## 4. Konsumsi Spec Resolution API
Endpoint yang dipakai, caching, workspace scoping.

## 5. Status Implementasi Hari Ini
Gap terhadap kontrak spec frontend (diisi dan diperbarui per rilis).
