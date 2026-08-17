import { defineConfig } from "vitepress"
import { fileURLToPath } from "node:url"

// Project root = docs-site/ ; konten Markdown live di ../docs (single source of truth)
// via symlink docs-site/docs -> ../docs (dibuat saat setup). Pakai symlink supaya
// resolusi modul Vite/Vue berjalan dari docs-site/node_modules.
// Config file berada di docs-site/.vitepress/, jadi path dihitung relatif dari situ.
const docsDir = fileURLToPath(new URL("../docs", import.meta.url))
const outDir = fileURLToPath(new URL("../dist", import.meta.url))
// Catatan: public dir VitePress default = <root>/public (docs-site/public),
// tidak perlu (dan tidak bisa) di-set lewat top-level config.

// Sidebar item helper
const item = (text: string, link: string) => ({ text, link })

// Docs source menulis link ke folder-index sebagai "x/README.md" — bentuk yang
// klikable di editor (VS Code) dan GitHub. VitePress hanya memperlakukan
// index.md sebagai folder route, bukan README.md, sehingga "x/README.md" tanpa
// perlakuan khusus akan dirender "x/README" → 404. Helper ini menormalkan href
// ke bentuk folder ("x/README.md" → "x/", "../README.md" → "../", dst.) sebelum
// linkPlugin milik VitePress menormalisasi & mendaftarkan dead-link.
function fixReadmeHref(href: string): string {
  // Link eksternal / protokol khusus tidak disentuh
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(href)) return href
  const hash = href.match(/#.*$/)?.[0] ?? ""
  const path = hash ? href.slice(0, -hash.length) : href
  const dir = path.match(/^(.*\/)README\.md$/i)?.[1]
  if (dir !== undefined) return dir + hash
  if (/^README\.md$/i.test(path)) return "./" + hash
  return href
}

const specBackend = [
  item("Backend Spec", "/spec/backend/"),
  item("Core Basic", "/spec/backend/01-core-basic"),
  item("Core Extended", "/spec/backend/02-core-extended"),
  item("Entity Extension", "/spec/backend/03-entity-extension"),
  item("PersistBackend", "/spec/backend/04-persist-backend"),
  item("Field Types", "/spec/backend/05-field-types"),
  item("Script Runtime", "/spec/backend/06-script-runtime"),
]

const specFrontend = [
  item("Frontend Spec", "/spec/frontend/"),
  item("Visual Hierarchy", "/spec/frontend/01-visual-hierarchy"),
  item("VisualSpecKind", "/spec/frontend/02-visual-spec-kind"),
  item("Renderer Kind", "/spec/frontend/03-renderer-kind"),
  item("Spec Resolution API", "/spec/frontend/04-spec-resolution-api"),
  item("App Kinds", "/spec/frontend/05-app-kinds"),
  item("Page Kinds", "/spec/frontend/06-page-kinds"),
  item("Component Kinds", "/spec/frontend/07-component-kinds"),
  item("FormSpecExpr", "/spec/frontend/08-formspec-expr"),
]

const specPlatform = [
  item("Platform Spec", "/spec/platform/"),
  item("Overview", "/spec/platform/01-overview"),
  item("Workspace / App / Module", "/spec/platform/02-workspace-app-module"),
  item("Kind System", "/spec/platform/03-kind-system"),
  item("Control Plane", "/spec/platform/04-control-plane"),
  item("Plane Protocol", "/spec/platform/05-plane-protocol"),
  item("Datastore", "/spec/platform/06-datastore"),
  item("Marketplace", "/spec/platform/07-marketplace"),
  item("Project Layout", "/spec/platform/08-project-layout"),
  item("Observability", "/spec/platform/09-observability"),
  item("Deployment Operations", "/spec/platform/10-deployment-operations"),
]

const kindGroups = [
  item("Kind Reference", "/kind/"),
  {
    text: "Curation",
    collapsed: true,
    items: [
      item("App", "/kind/curation/App"),
      item("Module", "/kind/curation/Module"),
    ],
  },
  {
    text: "Data",
    collapsed: true,
    items: [
      item("Api", "/kind/data/Api"),
      item("Config", "/kind/data/Config"),
      item("Entity", "/kind/data/Entity"),
      item("Integrator", "/kind/data/Integrator"),
      item("KindDefinition", "/kind/data/KindDefinition"),
      item("Migration", "/kind/data/Migration"),
      item("Mockup", "/kind/data/Mockup"),
      item("Service", "/kind/data/Service"),
      item("Subscription", "/kind/data/Subscription"),
      item("Webhook", "/kind/data/Webhook"),
      item("Workflow", "/kind/data/Workflow"),
    ],
  },
  {
    text: "Infra",
    collapsed: true,
    items: [
      item("Datastore", "/kind/infra/Datastore"),
      item("Environment", "/kind/infra/Environment"),
      item("PersistBackend", "/kind/infra/PersistBackend"),
      item("Policy", "/kind/infra/Policy"),
      item("Renderer", "/kind/infra/Renderer"),
    ],
  },
  {
    text: "UI",
    collapsed: true,
    items: [
      item("ApprovalInbox", "/kind/ui/ApprovalInbox"),
      item("Calendar", "/kind/ui/Calendar"),
      item("Dashboard", "/kind/ui/Dashboard"),
      item("Form", "/kind/ui/Form"),
      item("Kanban", "/kind/ui/Kanban"),
      item("Listing", "/kind/ui/Listing"),
      item("NotificationCenter", "/kind/ui/NotificationCenter"),
      item("Page", "/kind/ui/Page"),
      item("Print", "/kind/ui/Print"),
      item("Report", "/kind/ui/Report"),
      item("Table", "/kind/ui/Table"),
      item("Theme", "/kind/ui/Theme"),
      item("Timeline", "/kind/ui/Timeline"),
      item("Widget", "/kind/ui/Widget"),
      item("Wizard", "/kind/ui/Wizard"),
    ],
  },
]

const renderers = [
  item("Renderers", "/renderers/"),
  item("Realtime", "/renderers/realtime"),
  {
    text: "shadcn-shell",
    collapsed: true,
    items: [
      item("Overview", "/renderers/shadcn-shell/"),
      item("Architecture", "/renderers/shadcn-shell/01-architecture"),
      item("Derivation Engine", "/renderers/shadcn-shell/02-derivation-engine"),
      item("Kind Renderers", "/renderers/shadcn-shell/03-kind-renderers"),
      item("Theming & Assets", "/renderers/shadcn-shell/04-theming-assets"),
    ],
  },
  {
    text: "jsonb-persist",
    collapsed: true,
    items: [
      item("Overview", "/renderers/jsonb-persist/"),
      item("Architecture", "/renderers/jsonb-persist/01-architecture"),
      item(
        "Schema Strategies",
        "/renderers/jsonb-persist/02-schema-strategies",
      ),
      item("Migration Engine", "/renderers/jsonb-persist/03-migration-engine"),
      item("Query & Keys", "/renderers/jsonb-persist/04-query-and-keys"),
    ],
  },
]

const architecture = [
  item("Architecture", "/architecture/"),
  item("Architecture Overview", "/architecture/01-architecture-overview"),
  item("Admin Surfaces", "/architecture/02-admin-surfaces"),
  item("Deployment Flow", "/architecture/03-deployment-flow"),
  item("Resource Registration", "/architecture/04-resource-registration"),
  item("Failover", "/architecture/05-failover"),
  item("K8s Operator", "/architecture/06-k8s-operator"),
  item("Vertical Modules", "/architecture/07-vertical-modules"),
  item("Repo Structure", "/architecture/08-repo-structure"),
]

const runtimes = [
  item("Runtimes", "/runtimes/"),
  item("formspec-ctl", "/runtimes/01-formspec-ctl"),
  item("formspec-resource", "/runtimes/02-formspec-resource"),
  item("formspec-operator", "/runtimes/03-formspec-operator"),
  item("formspec-sidecar", "/runtimes/04-formspec-sidecar"),
  item("Engine API Layer", "/runtimes/05-engine-api-layer"),
]

const cliTools = [
  item("CLI Tools", "/cli-tools/"),
  item("formspec dev", "/cli-tools/01-formspec-dev"),
  item("formspec CLI", "/cli-tools/02-formspec-cli"),
  item("formspec generate", "/cli-tools/03-formspec-generate"),
  item("formspec-ctl", "/cli-tools/04-formspec-ctl"),
  item("formspec consult", "/cli-tools/05-formspec-consult"),
]

const guides = [
  item("Guides", "/guides/"),
  item("How to Run", "/guides/how-to-run"),
  item("Order-to-Cash Tutorial", "/guides/order-to-cash-tutorial"),
  item("Order-to-Cash Companion", "/guides/order-to-cash-companion"),
  item("Agent-Assisted App Dev", "/guides/agent-assisted-app-development"),
  item("Authoring a Shell", "/guides/authoring-a-shell"),
  item("Authoring a Page Renderer", "/guides/authoring-a-page-renderer"),
  item("Authoring a PersistBackend", "/guides/authoring-a-persist-backend"),
]

const ai = [
  item("FormSpec AI", "/ai/"),
  item("AI Architecture", "/ai/01-architecture"),
  item("formspec consult", "/ai/02-formspec-consult"),
  item("Local MCP", "/ai/03-formspec-local-mcp"),
  item("Remote MCP", "/ai/04-formspec-remote-mcp"),
  item("LLM Provider Layer", "/ai/05-llm-provider-layer"),
  item("FormSpec Skill", "/ai/06-formspec-skill"),
]

const reference = [
  item("Reference", "/reference/"),
  item("Glossary", "/reference/glossary"),
]

const comparison = [
  item("Comparison", "/comparison/"),
  item("vs Frappe", "/comparison/formspec-vs-frappe"),
  item("vs Laravel", "/comparison/formspec-vs-laravel"),
  item("vs Vercel", "/comparison/formspec-vs-vercel"),
  item("vs Budibase", "/comparison/formspec-vs-budibase"),
  item("vs PocketBase", "/comparison/formspec-vs-pocketbase"),
  item("vs Supabase", "/comparison/formspec-vs-supabase"),
  item("vs Spring Boot", "/comparison/formspec-vs-springboot"),
  item("vs Custom App", "/comparison/formspec-vs-custom-app"),
]

export default defineConfig({
  title: "FormSpec",
  description:
    "Dokumentasi FormSpec — ekosistem spec-first untuk aplikasi bisnis",
  lang: "id-ID",
  cleanUrls: true,
  srcDir: docsDir,
  outDir,
  // Docs source memakai README.md sebagai index tiap folder. VitePress tidak
  // otomatis memetakan README.md → index folder, jadi rewrite eksplisit:
  //   README.md            → index.md          (home /)
  //   spec/README.md       → spec/index.md     (/spec/)
  //   spec/platform/README.md → spec/platform/index.md  (/spec/platform/)
  rewrites: (id) => id.replace(/README\.md$/, "index.md"),
  // srcDir adalah symlink docs-site/docs -> ../docs. preserveSymlinks=true
  // membuat Vite meresolusi modul (vue, vue/server-renderer, dst.) dari
  // lokasi symlink (docs-site/) sehingga node_modules ditemukan.
  vite: {
    resolve: {
      preserveSymlinks: true,
    },
  },
  srcExclude: [
    "plan/**",
    "changelog/**",
    "presentations/**",
    "technical-notes/**",
    "**/docs_old/**",
  ],
  lastUpdated: true,
  // Docs banyak memakai placeholder angle-bracket (mis. `<name>`, `<id>`,
  // `<module>`) di dalam prosa. Dengan html:false, markdown-it meng-escape
  // tag yang tidak dikenal → placeholder tampil literal (sesuai niat penulis)
  // tanpa gagal di compiler Vue. Mermaid fence & tabel markdown tidak
  // terpengaruh (keduanya diproses terpisah dari raw HTML).
  markdown: {
    html: false,
    config(md) {
      // Bungkus rule link_open (milik VitePress) supaya href "x/README.md"
      // dikoreksi ke "x/" sebelum linkPlugin menormalisasi URL dan mencatat
      // dead-link. Dengan begitu:
      //   - source tetap pakai "x/README.md" → klikable di editor & GitHub
      //   - output site benar "x/" → tidak 404 (rewrites README.md → index.md)
      const originalLinkOpen =
        md.renderer.rules.link_open ??
        ((tokens, idx, opts, env, self) => self.renderToken(tokens, idx, opts))
      md.renderer.rules.link_open = (tokens, idx, opts, env, self) => {
        const token = tokens[idx]
        const hrefIndex = token.attrIndex("href")
        if (hrefIndex >= 0 && token.attrs) {
          token.attrs[hrefIndex][1] = fixReadmeHref(token.attrs[hrefIndex][1])
        }
        return originalLinkOpen(tokens, idx, opts, env, self)
      }
    },
  },
  // Cek dead-link tetap aktif untuk error nyata, tapi toleransi kategori
  // berikut (folder internal yang memang tidak ikut di-site, konvensi
  // README-as-index, sisa rename forma→formspec, dan localhost dev):
  ignoreDeadLinks: [
    (link: string) => {
      // Folder internal / referensi luar docs yang tidak ada di site
      if (
        /(^|\/)(plan|changelog|presentations|technical-notes|docs_old)\//.test(
          link,
        )
      )
        return true
      if (/(^|\/)(examples|ai_skills|scripts|verticals|docs_old)\//.test(link))
        return true
      // Konvensi: docs menaut "./x/index" padahal index-nya README.md
      // (VitePress memetakan README.md → root folder). Bukan kesalahan site.
      if (/\/index$/.test(link)) return true
      // Sisa rename forma→formspec (belum selesai di docs source)
      if (/08-formaexpr/.test(link)) return true
      // Link relatif stale di docs source: kind docs (docs/kind/<group>/)
      // menaut "../spec/..." tapi kedalaman seharusnya "../../spec/...".
      // Fix sebenarnya di generator kind-docs (di luar scope docs-site);
      // di sini di-toleransi supaya build tidak gagal.
      if (/\.\.\/spec\//.test(link)) return true
      // Stale depth serupa di spec/renderers → dirujuk dengan satu "../" kurang.
      if (/\.\.\/renderers\/realtime/.test(link)) return true
      if (/\.\.\/runtimes\/04-formspec-sidecar/.test(link)) return true
      // Link dev lokal (http://localhost:*) di docs
      if (/^http:\/\/localhost/.test(link)) return true
      return false
    },
  ],
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ["meta", { property: "og:title", content: "FormSpec Docs" }],
    [
      "meta",
      {
        property: "og:description",
        content:
          "Dokumentasi FormSpec — ekosistem spec-first untuk aplikasi bisnis",
      },
    ],
  ],
  themeConfig: {
    logo: "/favicon.svg",
    // Brand (logo + "FormSpec") di kiri atas → landing site, bukan root docs
    logoLink: "https://formspec.dev",
    nav: [
      { text: "Spec", link: "/spec/", activeMatch: "/spec/" },
      { text: "Kinds", link: "/kind/", activeMatch: "/kind/" },
      { text: "Renderers", link: "/renderers/", activeMatch: "/renderers/" },
      {
        text: "Arsitektur",
        link: "/architecture/",
        activeMatch: "/architecture/",
      },
      { text: "Guides", link: "/guides/", activeMatch: "/guides/" },
      {
        text: "Lainnya",
        items: [
          { text: "Runtimes", link: "/runtimes/" },
          { text: "CLI Tools", link: "/cli-tools/" },
          { text: "FormSpec AI", link: "/ai/" },
          { text: "Reference", link: "/reference/" },
          { text: "Comparison", link: "/comparison/" },
        ],
      },
      { text: "Landing", link: "https://formspec.dev" },
      {
        text: "GitHub",
        link: "https://github.com/primadi/formspec",
      },
    ],
    sidebar: {
      "/spec/backend/": specBackend,
      "/spec/frontend/": specFrontend,
      "/spec/platform/": specPlatform,
      "/spec/": [...specPlatform, ...specBackend, ...specFrontend],
      "/kind/": kindGroups,
      "/renderers/": renderers,
      "/architecture/": architecture,
      "/runtimes/": runtimes,
      "/cli-tools/": cliTools,
      "/guides/": guides,
      "/ai/": ai,
      "/reference/": reference,
      "/comparison/": comparison,
      "/": [
        item("Dokumentasi FormSpec", "/"),
        {
          text: "Spec",
          collapsed: false,
          items: [...specPlatform, ...specBackend, ...specFrontend],
        },
        { text: "Kinds", collapsed: false, items: kindGroups },
        { text: "Renderers", collapsed: false, items: renderers },
        { text: "Architecture", collapsed: false, items: architecture },
        { text: "Runtimes", collapsed: false, items: runtimes },
        { text: "CLI Tools", collapsed: false, items: cliTools },
        { text: "Guides", collapsed: false, items: guides },
        { text: "AI", collapsed: false, items: ai },
        { text: "Reference", collapsed: false, items: reference },
        { text: "Comparison", collapsed: false, items: comparison },
      ],
    },
    search: {
      provider: "local",
    },
    footer: {
      message: "Standar terbuka (CC0) dengan implementasi referensi.",
      copyright: "© 2026 FormSpec",
    },
    socialLinks: [
      { icon: "github", link: "https://github.com/primadi/formspec" },
    ],
  },
})
