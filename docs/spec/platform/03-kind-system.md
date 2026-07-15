# Kind System

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Taksonomi Kind
Peta seluruh kind: kind data/perilaku (Document, Service, Action, Workflow, …),
kind visual (instance dari VisualSpecKind, hirarkis — bukan flat), kind
infrastruktur (Datastore, Environment), kind kurasi (App, Module).

## 2. Meta-Kinds
Kind yang mendeklarasikan kind lain:
- **KindDefinition** — kind kustom data.
- **VisualSpecKind** — mendeklarasikan jenis view baru + skema + renderer
  contract (detail di spec frontend).
- **Renderer** — implementasi konkret sebuah VisualSpecKind (detail di spec
  frontend).
- **PersistBackend** — implementasi penyimpanan (detail di spec backend).

## 3. Katalog Concern → Kind
Tabel "kebutuhan umum aplikasi bisnis → kind yang menjawabnya".

## 4. Lampiran: Pemetaan Kind → Plane
Tabel otoritatif kind mana hidup di plane mana (resource plane, control plane),
termasuk baris untuk VisualSpecKind, Renderer, dan PersistBackend.
