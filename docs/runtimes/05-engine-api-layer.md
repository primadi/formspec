# Engine API Layer

**Updated:** 2026-07-15 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap dari kode
> `internal/api/`.

## 1. Peran
Lapisan HTTP runtime engine: routing workspace-scoped, handler CRUD per entity,
endpoint meta, websocket hub.

## 2. Router & Middleware
Struktur route, auth, permission.

## 3. Endpoint Meta
Penyajian manifest untuk shell — implementasi dari kontrak Spec Resolution API
(spec frontend 04).

## 4. WebSocket & Event Delivery
Hub, subscription, pengiriman event ke klien.

## 5. Status Implementasi Hari Ini
Cakupan dan gap.
