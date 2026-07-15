# Renderer

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Peran
Implementasi konkret sebuah VisualSpecKind. Satu VisualSpecKind bisa punya
banyak renderer (filosofi UX berbeda, stack berbeda).

```yaml
apiVersion: forma/v1
kind: Renderer
metadata:
  name: kanban-vue-community
spec:
  implements: kanban
  stack_family: vue
  trust_tier: community
```

## 2. Field
`implements` (nama VisualSpecKind), `stack_family` (kecocokan shell),
`trust_tier` (official/verified/community — seragam dengan marketplace).

## 3. Registry & Resolusi
Bagaimana engine memilih renderer untuk sebuah instance di App tertentu;
default resmi vs override.

## 4. Renderer = Interpreter Runtime
Renderer komunitas pun interpreter runtime yang mengonsumsi Spec Resolution
API — bukan build step per kombinasi spec+renderer.

## 5. Konformansi
Bagaimana renderer diverifikasi memenuhi `renderer_contract` VisualSpecKind-nya
(validasi statis skema vs test-suite konformansi — keputusan dicatat di sini).
