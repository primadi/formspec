// ─── Renderer Preloader ───
//
// Eagerly imports the most frequently used kind renderers so their chunks
// start downloading in the background during boot, reducing perceived
// latency on first navigation.
//
// Calling `preloadCommonRenderers()` early in the app lifecycle triggers
// Vite to fetch these modules *before* the user clicks a menu item.

export function preloadCommonRenderers(): void {
  // These dynamic imports are deliberately not awaited — we just want to
  // kick off the HTTP request for the chunk. React.lazy in router.tsx
  // will reuse the already-loading/loaded module.
  import("@/kinds/table/TableRenderer")
  import("@/kinds/form/FormRenderer")
  import("@/kinds/page/DetailPage")
  import("@/kinds/page/PageRenderer")
}
