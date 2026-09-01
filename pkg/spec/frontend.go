package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ─── Frontend Kinds (Frontend Spec §2–§13) ───

// IsPublic returns the effective public flag — nil or true means the view
// has a route (derived Page wrapper); explicit false means embed-only.
func IsPublic(p *bool) bool { return p == nil || *p }

// ContextDecl declares one render-context variable injected into a Page/Form
// (render-context-standard plan). Sources are a closed set:
//
//	session | entity | api | const | expr
//
// Standard slots (`user`, `route`, `fields`) are always present and need no
// declaration; `context:` adds named variables resolved from these sources.
type ContextDecl struct {
	// Name is the context variable name (e.g. "vendor") — referenced in
	// templates as {vendor.field}.
	Name string `yaml:"name" json:"name"`
	// Source is the data source — closed set.
	// @schema {description: "Context source — closed set.", enum: ["session", "entity", "api", "const", "expr"]}
	Source string `yaml:"source" json:"source"`
	// Entity ref for source: entity ("module.entity").
	Entity string `yaml:"entity,omitempty" json:"entity,omitempty"`
	// ID for source: entity — a record id, or a template token like
	// "{user.vendor_id}" resolved against the standard slots.
	ID string `yaml:"id,omitempty" json:"id,omitempty"`
	// Call for source: api — "module.service.action".
	Call string `yaml:"call,omitempty" json:"call,omitempty"`
	// Params for source: api — action params.
	Params map[string]any `yaml:"params,omitempty" json:"params,omitempty"`
	// Value for source: const — a literal value.
	Value any `yaml:"value,omitempty" json:"value,omitempty"`
	// Expr for source: expr — a FormSpecExpr string evaluated against the
	// standard slots + previously-resolved context entries.
	Expr string `yaml:"expr,omitempty" json:"expr,omitempty"`
	// Realtime (source: entity) — subscribe to the entity's events and
	// re-resolve the record on change (dashboard live-update, Phase 3).
	Realtime bool `yaml:"realtime,omitempty" json:"realtime,omitempty"`
	// Fallback is used while the source is loading or on error.
	Fallback any `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

// ContextSourceSet is the closed set of allowed context sources.
var ContextSourceSet = map[string]bool{
	"session": true,
	"entity":  true,
	"api":     true,
	"const":   true,
	"expr":    true,
}

// ValidateContextDecls validates a list of context declarations: unique
// names and a closed source set.
func ValidateContextDecls(decls []ContextDecl) error {
	seen := map[string]bool{}
	for i, d := range decls {
		if d.Name == "" {
			return fmt.Errorf("context[%d]: `name` is required", i)
		}
		if seen[d.Name] {
			return fmt.Errorf("context: duplicate name %q", d.Name)
		}
		seen[d.Name] = true
		if !ContextSourceSet[d.Source] {
			return fmt.Errorf("context %q: source %q is not a known source (closed set: session, entity, api, const, expr)", d.Name, d.Source)
		}
		switch d.Source {
		case "entity":
			if d.Entity == "" {
				return fmt.Errorf("context %q: source entity requires `entity` (module.entity)", d.Name)
			}
		case "api":
			if d.Call == "" {
				return fmt.Errorf("context %q: source api requires `call` (module.service.action)", d.Name)
			}
		case "expr":
			if d.Expr == "" {
				return fmt.Errorf("context %q: source expr requires `expr`", d.Name)
			}
		}
	}
	return nil
}

// PageSpec defines a routed screen composing blocks (Frontend §3).
// `blocks` and `tabs` are mutually exclusive; `mode: custom` excludes both.
type PageSpec struct {
	// @schema {description: "If true (default), a route is generated for this page. Set false to restrict to embedding only.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "/orders/:id"}
	Route string `yaml:"route" json:"route"`
	// @schema {example: "Order {order.number}"}
	Title string `yaml:"title" json:"title"`
	// TitleVisible hides the rendered page-title heading when false (default
	// true). Pages whose first block carries its own display title (e.g. a
	// hero section) set title_visible: false to avoid a duplicated heading.
	// @schema {description: "Render the page title heading (default true)"}
	TitleVisible *bool       `yaml:"title_visible,omitempty" json:"title_visible,omitempty"`
	Icon         string      `yaml:"icon,omitempty" json:"icon,omitempty"`
	Description  string      `yaml:"description,omitempty" json:"description,omitempty"`
	Permissions  []string    `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Blocks       []PageBlock `yaml:"blocks,omitempty" json:"blocks,omitempty"`
	Tabs         []PageTab   `yaml:"tabs,omitempty" json:"tabs,omitempty"`
	Layout       *PageLayout `yaml:"layout,omitempty" json:"layout,omitempty"`
	// Mode is "custom" for a full-code page that hands all rendering to an
	// asset (frontend/06-page-kinds.md §13). Empty means blocks/tabs.
	// @schema {description: "Page mode. `custom` hands all rendering to an asset component; empty means blocks/tabs composition.", enum: ["", "custom"]}
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
	// Asset is the spec-root-relative asset path for `mode: custom`
	// (e.g. "modules/logistics/assets/dispatch-console.js").
	Asset string `yaml:"asset,omitempty" json:"asset,omitempty"`
	// Binds is the backend footprint (entities/actions/subscribe) a custom
	// page may touch — enforced client-side like a component's `needs`.
	Binds *PageBinds `yaml:"binds,omitempty" json:"binds,omitempty"`
	// Context declares render-context variables injected into this page's
	// blocks (render-context-standard plan). Standard slots (`user`, `route`)
	// are always present; `context:` adds named variables from a closed set
	// of sources.
	Context []ContextDecl `yaml:"context,omitempty" json:"context,omitempty"`
	// Renderer is the per-instance renderer override (frontend/03-renderer-
	// kind.md §3), e.g. "community/super-kanban". Overrides the App-level
	// `renderers:` map for this instance.
	Renderer string `yaml:"renderer,omitempty" json:"renderer,omitempty"`
}

// PageBinds declares the backend footprint of a `mode: custom` page
// (frontend/06-page-kinds.md §13) — the same role as `needs:` on a component
// block (frontend/07-component-kinds.md §4). Calls outside `binds` fail
// client-side and are never authorized server-side either.
type PageBinds struct {
	// Entities the page may read/write, as "module.entity" refs.
	Entities []string `yaml:"entities,omitempty" json:"entities,omitempty"`
	// Actions the page may invoke, as "module.entity.action" (or wildcard).
	Actions []string `yaml:"actions,omitempty" json:"actions,omitempty"`
	// Subscribe channels (entity refs) the page may subscribe to.
	Subscribe []string `yaml:"subscribe,omitempty" json:"subscribe,omitempty"`
}

// PageLayout controls block arrangement on a page.
type PageLayout struct {
	Columns int `yaml:"columns,omitempty" json:"columns,omitempty"`
	// Mode "split" arranges a master list block and a detail block side by
	// side (frontend/06-page-kinds.md §1.1). Empty means normal vertical flow.
	// @schema {description: "Layout mode. `split` = master-detail (master list left, detail right).", enum: ["", "split"]}
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

// PageBlock is a compositional unit within a Page.
type PageBlock struct {
	Form      *BlockRef     `yaml:"form,omitempty" json:"form,omitempty"`
	Table     *BlockRef     `yaml:"table,omitempty" json:"table,omitempty"`
	Component *BlockRef     `yaml:"component,omitempty" json:"component,omitempty"`
	Widget    *BlockRef     `yaml:"widget,omitempty" json:"widget,omitempty"`
	HTML      string        `yaml:"html,omitempty" json:"html,omitempty"`
	Section   *SectionBlock `yaml:"section,omitempty" json:"section,omitempty"`
}

// SectionBlock is a declarative presentation section inside a Page
// (frontend/06-page-kinds.md §1). Sections are generic — reusable in any App
// (marketing/hero, no-nav, sidebar-nav, ...): pure presentation, no data
// binding, no auth, and zero styling fields — every visual token lives in
// kind: Theme (frontend/05-app-kinds.md §5), never inline.
type SectionBlock struct {
	// @schema {description: "Section block type — closed set.", enum: ["hero", "feature_grid", "card", "carousel", "cta"]}
	Type string `yaml:"type" json:"type"`
	// @schema {example: "Kelola klinik Anda"}
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	// @schema {example: "Satu platform untuk jadwal, pasien, dan tagihan."}
	Subtitle string `yaml:"subtitle,omitempty" json:"subtitle,omitempty"`
	// @schema {example: "assets/hero.png"}
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
	// @schema {example: "primary"}
	Variant string      `yaml:"variant,omitempty" json:"variant,omitempty"` // hero/cta: primary | secondary | ghost
	CTA     *SectionCTA `yaml:"cta,omitempty" json:"cta,omitempty"`
	// Items is the content list for feature_grid / card / carousel.
	Items []SectionItem `yaml:"items,omitempty" json:"items,omitempty"`
	// Columns controls the grid width for feature_grid / card (default 3).
	Columns int `yaml:"columns,omitempty" json:"columns,omitempty"`
	// Align controls text alignment of title/subtitle/items — left (default)
	// | center | right. Ignored by cta (always centered by design).
	// @schema {enum: ["left", "center", "right"], default: "left"}
	Align string `yaml:"align,omitempty" json:"align,omitempty"`
	// Autoplay + IntervalMS drive carousel auto-advance (default off / 5000ms).
	Autoplay   bool `yaml:"autoplay,omitempty" json:"autoplay,omitempty"`
	IntervalMS int  `yaml:"interval_ms,omitempty" json:"interval_ms,omitempty"`
}

// SectionCTA is one call-to-action link inside a section block.
type SectionCTA struct {
	// @schema {example: "Mulai"}
	Label string `yaml:"label" json:"label"`
	// @schema {example: "/app"}
	Href string `yaml:"href" json:"href"`
	// @schema {example: "primary", enum: ["primary", "secondary", "ghost"]}
	Variant string `yaml:"variant,omitempty" json:"variant,omitempty"`
}

// SectionItem is one content entry in a feature_grid / card / carousel block.
type SectionItem struct {
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	Text  string `yaml:"text,omitempty" json:"text,omitempty"`
	// Icon is a lucide icon name (renderer-specific; closed to the shell's
	// icon library — frontend/03-kind-renderers.md §5).
	Icon  string      `yaml:"icon,omitempty" json:"icon,omitempty"`
	Image string      `yaml:"image,omitempty" json:"image,omitempty"`
	CTA   *SectionCTA `yaml:"cta,omitempty" json:"cta,omitempty"`
}

// PageTab is one tab on a tabbed page (Frontend §3, tabs variant).
type PageTab struct {
	Label     string    `yaml:"label" json:"label"`
	Form      *BlockRef `yaml:"form,omitempty" json:"form,omitempty"`
	Table     *BlockRef `yaml:"table,omitempty" json:"table,omitempty"`
	Component *BlockRef `yaml:"component,omitempty" json:"component,omitempty"`
}

// BlockRef references another resource within a page block.
// Entity is resolved from the referenced Form/Table manifest's `spec.entity` —
// not declared inline here (removed as redundant, since every Form/Table has
// a 1:1 mapping to an entity).
type BlockRef struct {
	Ref   string         `yaml:"ref,omitempty" json:"ref,omitempty"`
	Asset string         `yaml:"asset,omitempty" json:"asset,omitempty"` // component blocks (§7)
	ID    string         `yaml:"id,omitempty" json:"id,omitempty"`
	Mode  string         `yaml:"mode,omitempty" json:"mode,omitempty"` // view | edit
	Param map[string]any `yaml:"param,omitempty" json:"param,omitempty"`
	Props map[string]any `yaml:"props,omitempty" json:"props,omitempty"` // component blocks
	// Binds links a detail block to a master list block in a split layout
	// (frontend/06-page-kinds.md §1.1) — the detail follows the master's
	// row selection instead of a `:id` route param.
	Binds *BlockBinds `yaml:"binds,omitempty" json:"binds,omitempty"`
}

// BlockBinds ties a detail block to the master list block that drives it in a
// `layout.mode: split` page (frontend/06-page-kinds.md §1.1).
type BlockBinds struct {
	// Source is the `ref` of the master Table block whose row selection
	// drives this detail block.
	Source string `yaml:"source" json:"source"`
	// Param is the field of the selected master record injected into the
	// detail block as its record id (usually "id").
	Param string `yaml:"param" json:"param"`
}

// FormSpec defines an input/edit layout (Frontend §4).
type FormSpec struct {
	// @schema {description: "If true (default), a route /module/form/<name> is auto-generated. Set false for embed-only forms.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "billing.order"}
	Entity string `yaml:"entity" json:"entity"`
	// @schema {example: "edit", enum: ["create", "edit", "view"]}
	Mode     string          `yaml:"mode,omitempty" json:"mode,omitempty"` // create | edit | view
	Sections []FormSection   `yaml:"sections" json:"sections"`
	Actions  []FormAction    `yaml:"actions,omitempty" json:"actions,omitempty"`
	Submit   *FormSubmit     `yaml:"submit,omitempty" json:"submit,omitempty"`
	Render   *FormRenderDecl `yaml:"render,omitempty" json:"render,omitempty"`
	// Context declares render-context variables injected into this form's
	// expressions (visible_when/required_when/compute). Standard slots
	// (`user`, `route`, `fields`) are always present.
	Context []ContextDecl `yaml:"context,omitempty" json:"context,omitempty"`
}

// FormSection groups form fields.
type FormSection struct {
	Title       string      `yaml:"title" json:"title"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Columns     int         `yaml:"columns,omitempty" json:"columns,omitempty"`
	VisibleWhen string      `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	Fields      []FormField `yaml:"fields" json:"fields"`
}

// FormField configures a single field in a form.
// YAML uses `field:` to reference the Entity field name; JSON serializes as
// `name:` to match the TypeScript FormField interface.
type FormField struct {
	Field        string `yaml:"field" json:"name"`
	Label        string `yaml:"label,omitempty" json:"label,omitempty"`
	Placeholder  string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Help         string `yaml:"help,omitempty" json:"help,omitempty"`
	Widget       string `yaml:"widget,omitempty" json:"widget,omitempty"`
	ReadOnly     bool   `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	ReadonlyWhen string `yaml:"readonly_when,omitempty" json:"readonly_when,omitempty"`
	RequiredWhen string `yaml:"required_when,omitempty" json:"required_when,omitempty"`
	VisibleWhen  string `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	Compute      string `yaml:"compute,omitempty" json:"compute,omitempty"`
}

// FormAction is a custom action button on a form (Frontend §4).
type FormAction struct {
	Action  string `yaml:"action" json:"action"`
	Label   string `yaml:"label,omitempty" json:"label,omitempty"`
	Style   string `yaml:"style,omitempty" json:"style,omitempty"` // primary | secondary | danger
	Confirm string `yaml:"confirm,omitempty" json:"confirm,omitempty"`
}

// FormSubmit configures the submit button and post-submit behavior.
type FormSubmit struct {
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Redirect string `yaml:"redirect,omitempty" json:"redirect,omitempty"`
	Message  string `yaml:"message,omitempty" json:"message,omitempty"`
}

// FormRender controls how a form is displayed (Frontend §1.6 — design-time locking).
// @schema {description: "Render mode: modal (popup dialog), drawer (side panel), separate_page (full page)", enum: ["modal", "drawer", "separate_page"]}
type FormRender string

// FormRenderDecl is the design-time container declaration of a Form
// (Frontend §1.6). YAML accepts both `render: separate_page` (shorthand) and
// `render: { mode: separate_page }`; JSON always serializes the object form to
// match the renderer contract (`spec.render.mode`).
type FormRenderDecl struct {
	// @schema {description: "Render mode: modal (popup dialog), drawer (side panel), separate_page (full page)", enum: ["modal", "drawer", "separate_page"]}
	Mode FormRender `yaml:"mode" json:"mode"`
}

// UnmarshalYAML accepts either the scalar shorthand ("separate_page") or the
// object form ({mode: "separate_page"}).
func (d *FormRenderDecl) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		d.Mode = FormRender(s)
		return nil
	}
	type plain FormRenderDecl
	return value.Decode((*plain)(d))
}

// TableSpec defines a list/browse view (Frontend §5).
type TableSpec struct {
	// @schema {description: "If true (default), a route /module/table/<name> is auto-generated. Set false for embed-only tables.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "billing.order"}
	Entity  string        `yaml:"entity" json:"entity"`
	Columns []TableColumn `yaml:"columns" json:"columns"`
	// @schema {example: "-created_at"}
	DefaultSort  string        `yaml:"default_sort,omitempty" json:"default_sort,omitempty"`
	PageSize     int           `yaml:"page_size,omitempty" json:"page_size,omitempty"`
	Search       bool          `yaml:"search,omitempty" json:"search,omitempty"`
	Realtime     bool          `yaml:"realtime,omitempty" json:"realtime,omitempty"`
	RowActions   []TableAction `yaml:"row_actions,omitempty" json:"row_actions,omitempty"`
	BulkActions  []TableAction `yaml:"bulk_actions,omitempty" json:"bulk_actions,omitempty"`
	Filters      []FilterSpec  `yaml:"filters,omitempty" json:"filters,omitempty"`
	FixedFilters []FilterSpec  `yaml:"fixed_filters,omitempty" json:"fixed_filters,omitempty"`
	// @schema {description: "Inline editing: cells editable in place for fields whose rules allow it (not readonly/computed/immutable, within update permission). Commit = per-row update with CAS version."}
	InlineEdit bool `yaml:"inline_edit,omitempty" json:"inline_edit,omitempty"`
	// @schema {description: "Batch editing: fields editable across a multi-row selection. Framework runs update per row, partial failure reported per row."}
	BatchEdit []string `yaml:"batch_edit,omitempty" json:"batch_edit,omitempty"`
}

// TableColumn configures a table column.
type TableColumn struct {
	Field    string `yaml:"field" json:"field"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Sortable bool   `yaml:"sortable,omitempty" json:"sortable,omitempty"`
	Width    string `yaml:"width,omitempty" json:"width,omitempty"`
	Align    string `yaml:"align,omitempty" json:"align,omitempty"`   // left | center | right
	Link     string `yaml:"link,omitempty" json:"link,omitempty"`     // Page name to navigate to
	Format   string `yaml:"format,omitempty" json:"format,omitempty"` // currency | date | relative | ...
	Widget   string `yaml:"widget,omitempty" json:"widget,omitempty"` // badge | ...
}

// TableAction is a clickable action on a table row or bulk selection.
type TableAction struct {
	Action     string `yaml:"action" json:"action"`
	Label      string `yaml:"label" json:"label"`
	Icon       string `yaml:"icon,omitempty" json:"icon,omitempty"`
	ConfirmMsg string `yaml:"confirm_msg,omitempty" json:"confirm_msg,omitempty"`
}

// FilterSpec is a generic filter declaration shared by every data kind that
// lists records (Table, Kanban, Listing, ApprovalInbox, …). A filter is used
// in one of two roles:
//   - `filters`: a user-adjustable control rendered in the kind's UI. If
//     `default` is set, the control is pre-seeded with that value (the user
//     can still change or clear it).
//   - `fixed_filters`: an immutable, server-side filter — always merged into
//     the list request, never rendered as a control and not user-clearable.
//
// The resolved value is sent to the list API as `field[op]=value`, so `op`
// defaults to "eq" and follows the backend filter operator set.
type FilterSpec struct {
	Field string `yaml:"field" json:"field"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// @schema {description: "Filter widget type", enum: ["text", "select", "date", "date_range"]}
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
	// @schema {description: "Filter operator sent to the list API", enum: ["eq", "neq", "gt", "gte", "lt", "lte", "between", "in", "nin", "like", "ilike", "null", "notnull"]}
	Op string `yaml:"op,omitempty" json:"op,omitempty"` // default "eq"
	// @schema {description: "Pre-set value for a user-adjustable filter. Supports \"today\" / \"today()\", resolved by the renderer as the server's current date."}
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	// @schema {description: "For select filters: show the \"All\" (clear) option. Default true."}
	ShowAll *bool `yaml:"show_all,omitempty" json:"show_all,omitempty"`
	// @schema {description: "For select filters: caption of the \"All\" (clear) option. Default \"(ALL)\"."}
	AllLabel string `yaml:"all_label,omitempty" json:"all_label,omitempty"`
}

// DashboardSpec defines a widget canvas (Frontend §5).
type DashboardSpec struct {
	// @schema {description: "If true (default), a route /module/dashboard/<name> is auto-generated.", example: "true"}
	Public       *bool             `yaml:"public,omitempty" json:"public,omitempty"`
	Title        string            `yaml:"title" json:"title"`
	Description  string            `yaml:"description,omitempty" json:"description,omitempty"`
	Customizable bool              `yaml:"customizable,omitempty" json:"customizable,omitempty"`
	Defaults     []string          `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	RefreshSecs  int               `yaml:"refresh,omitempty" json:"refresh,omitempty"`
	Realtime     bool              `yaml:"realtime,omitempty" json:"realtime,omitempty"`
	Widgets      []DashboardWidget `yaml:"widgets" json:"widgets"`
}

// DashboardWidget places a widget on a dashboard.
type DashboardWidget struct {
	Ref    string         `yaml:"ref" json:"ref"`
	Layout WidgetLayout   `yaml:"layout" json:"layout"`
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

// WidgetLayout controls grid placement.
type WidgetLayout struct {
	X int `yaml:"x" json:"x"`
	Y int `yaml:"y" json:"y"`
	W int `yaml:"w" json:"w"`
	H int `yaml:"h" json:"h"`
}

// WidgetSpec defines a reusable dashboard widget (Frontend §5).
type WidgetSpec struct {
	// @schema {description: "If true (default), a route /module/widget/<name> is auto-generated.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "Today's Revenue"}
	Title string `yaml:"title" json:"title"`
	// @schema {example: "metric", enum: ["metric", "chart", "table", "list"]}
	Type string `yaml:"type" json:"type"` // metric | chart | table | list
	// @schema {example: "sales-daily-summary"}
	Entity      string         `yaml:"entity,omitempty" json:"entity,omitempty"`
	Query       string         `yaml:"query,omitempty" json:"query,omitempty"`
	RefreshSecs int            `yaml:"refresh_secs,omitempty" json:"refresh_secs,omitempty"`
	Size        *WidgetLayout  `yaml:"size,omitempty" json:"size,omitempty"`
	Config      map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

// ReportSpec defines a parameterized tabular report (Frontend §5).
type ReportSpec struct {
	// @schema {description: "If true (default), a route /module/report/<name> is auto-generated.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "Sales by Category"}
	Title string `yaml:"title" json:"title"`
	// @schema {example: "billing.order"}
	Entity             string         `yaml:"entity" json:"entity"`
	RequiredPermission string         `yaml:"required_permission,omitempty" json:"required_permission,omitempty"`
	Parameters         []ReportParam  `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Columns            []ReportColumn `yaml:"columns" json:"columns"`
	Groups             []ReportGroup  `yaml:"groups,omitempty" json:"groups,omitempty"`
	Totals             []ReportTotal  `yaml:"totals,omitempty" json:"totals,omitempty"`
	Export             []string       `yaml:"export,omitempty" json:"export,omitempty"` // pdf | csv | xlsx
	// Source is the declarative parameterized filter (06-page-kinds.md §8
	// "Open — source.filter"). `filter` maps a list-API filter field to a
	// value that may be a `":param"` placeholder resolved from `parameters[]`
	// at execution time (e.g. `{ "transaction_date": ":from" }`).
	Source *ReportSource `yaml:"source,omitempty" json:"source,omitempty"`
}

// ReportSource declares the report's data source + parameterized filter.
type ReportSource struct {
	// @schema {example: "billing.order"}
	Entity string `yaml:"entity" json:"entity"`
	// Filter maps a list-API filter field to a literal value or a `":param"`
	// placeholder resolved from ReportSpec.Parameters at execution time.
	Filter map[string]string `yaml:"filter,omitempty" json:"filter,omitempty"`
}

// ReportParam is a filterable input for a report.
type ReportParam struct {
	Field    string `yaml:"field" json:"field"`
	Label    string `yaml:"label" json:"label"`
	Type     string `yaml:"type" json:"type"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default  any    `yaml:"default,omitempty" json:"default,omitempty"`
}

// ReportGroup defines a grouping level in a report.
type ReportGroup struct {
	Field string `yaml:"field" json:"field"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// ReportColumn is a column in a report output.
type ReportColumn struct {
	Field     string `yaml:"field" json:"field"`
	Label     string `yaml:"label" json:"label"`
	Aggregate string `yaml:"aggregate,omitempty" json:"aggregate,omitempty"`
	Format    string `yaml:"format,omitempty" json:"format,omitempty"`
}

// ReportTotal defines a total/aggregate row.
type ReportTotal struct {
	Label string `yaml:"label" json:"label"`
	Field string `yaml:"field" json:"field"`
	Fn    string `yaml:"fn" json:"fn"` // sum | avg | count | min | max
}

// WizardSpec defines a multi-step business process (Frontend §11).
type WizardSpec struct {
	// @schema {description: "If true (default), a route /module/wizard/<name> is auto-generated.", example: "true"}
	Public *bool  `yaml:"public,omitempty" json:"public,omitempty"`
	Title  string `yaml:"title" json:"title"`
	// @schema {example: "clinic.visit"}
	Entity     string            `yaml:"entity,omitempty" json:"entity,omitempty"`
	Action     string            `yaml:"action,omitempty" json:"action,omitempty"` // server action that commits all steps; if empty, final step plain-creates Entity
	OnComplete *WizardOnComplete `yaml:"on_complete,omitempty" json:"on_complete,omitempty"`
	Steps      []WizardStep      `yaml:"steps" json:"steps"`
}

// WizardOnComplete controls what happens after a successful final submit.
type WizardOnComplete struct {
	Restart  bool                `yaml:"restart,omitempty" json:"restart,omitempty"`
	Redirect string              `yaml:"redirect,omitempty" json:"redirect,omitempty"`
	Banner   []WizardSummaryItem `yaml:"banner,omitempty" json:"banner,omitempty"`
}

// WizardStep is one step in a multi-step wizard.
type WizardStep struct {
	Title        string              `yaml:"title" json:"title"`
	Description  string              `yaml:"description,omitempty" json:"description,omitempty"`
	Form         string              `yaml:"form,omitempty" json:"form,omitempty"`
	OnEnter      string              `yaml:"on_enter,omitempty" json:"on_enter,omitempty"` // fires when the step becomes active
	OnNext       string              `yaml:"on_next,omitempty" json:"on_next,omitempty"`   // fires on Next, before advancing
	OnPrev       string              `yaml:"on_prev,omitempty" json:"on_prev,omitempty"`   // fires on Previous, before going back
	Required     []string            `yaml:"required,omitempty" json:"required,omitempty"` // fields that gate the Next button
	DependsOn    string              `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Entity       string              `yaml:"entity,omitempty" json:"entity,omitempty"`
	Layout       string              `yaml:"layout,omitempty" json:"layout,omitempty"` // search_select | ...
	SearchFields []string            `yaml:"search_fields,omitempty" json:"search_fields,omitempty"`
	AllowCreate  bool                `yaml:"allow_create,omitempty" json:"allow_create,omitempty"`
	Fields       []FormField         `yaml:"fields,omitempty" json:"fields,omitempty"`
	Summary      []WizardSummaryItem `yaml:"summary,omitempty" json:"summary,omitempty"`
	Component    string              `yaml:"component,omitempty" json:"component,omitempty"`
}

// WizardSummaryItem is one line on a wizard confirmation step.
type WizardSummaryItem struct {
	Label string `yaml:"label" json:"label"`
	Field string `yaml:"field" json:"field"`
}

// KanbanSpec defines a drag-and-drop status board (Frontend §12).
type KanbanSpec struct {
	// @schema {description: "If true (default), a route /module/kanban/<name> is auto-generated.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "helpdesk.ticket"}
	Entity string `yaml:"entity" json:"entity"`
	// @schema {example: "status"}
	StatusField       string         `yaml:"status_field" json:"status_field"`
	Columns           []KanbanColumn `yaml:"columns" json:"columns"`
	CardTemplate      *KanbanCard    `yaml:"card_template,omitempty" json:"card_template,omitempty"`
	Realtime          *bool          `yaml:"realtime,omitempty" json:"realtime,omitempty"` // default true (§12)
	Filters           []FilterSpec   `yaml:"filters,omitempty" json:"filters,omitempty"`
	FixedFilters      []FilterSpec   `yaml:"fixed_filters,omitempty" json:"fixed_filters,omitempty"`
	Search            bool           `yaml:"search,omitempty" json:"search,omitempty"`
	RowActions        []TableAction  `yaml:"row_actions,omitempty" json:"row_actions,omitempty"`
	MaxCardsPerColumn int            `yaml:"max_cards_per_column,omitempty" json:"max_cards_per_column,omitempty"`
	Sortable          bool           `yaml:"sortable,omitempty" json:"sortable,omitempty"`             // enable within-column drag-to-reorder
	PositionField     string         `yaml:"position_field,omitempty" json:"position_field,omitempty"` // field storing user-adjustable position (e.g. "queue_position")
	// @schema {description: "FormSpecExpr pre-check UX before drop: evaluated against the record + target column; drop blocked when false. Server state-machine guard remains authority."}
	DragGuard string `yaml:"drag_guard,omitempty" json:"drag_guard,omitempty"`
}

// KanbanColumn is a status lane in a kanban board.
type KanbanColumn struct {
	Status string `yaml:"status" json:"status"`
	Label  string `yaml:"label" json:"label"`
	Color  string `yaml:"color,omitempty" json:"color,omitempty"`
}

// KanbanCard defines what fields appear on a card.
type KanbanCard struct {
	Title     string   `yaml:"title" json:"title"`
	Subtitle  string   `yaml:"subtitle,omitempty" json:"subtitle,omitempty"`
	Badge     string   `yaml:"badge,omitempty" json:"badge,omitempty"`
	Assignee  string   `yaml:"assignee,omitempty" json:"assignee,omitempty"`
	Fields    []string `yaml:"fields,omitempty" json:"fields,omitempty"`
	Component string   `yaml:"component,omitempty" json:"component,omitempty"`
}

// PrintSpec defines a printable document template (Frontend §9).
// One format per manifest — declared via `output.format` (Frontend §9).
type PrintSpec struct {
	// @schema {description: "If true (default), a route /module/print/<name> is auto-generated.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "billing.order"}
	Entity   string          `yaml:"entity" json:"entity"`
	Template string          `yaml:"template,omitempty" json:"template,omitempty"`
	Output   *PrintOutput    `yaml:"output,omitempty" json:"output,omitempty"`
	Header   *PrintHeader    `yaml:"header,omitempty" json:"header,omitempty"`
	Body     []PrintBodyItem `yaml:"body,omitempty" json:"body,omitempty"`
	Footer   *PrintFooter    `yaml:"footer,omitempty" json:"footer,omitempty"`
}

// PrintOutput selects the rendering pipeline and paper.
type PrintOutput struct {
	Format string      `yaml:"format" json:"format"` // pdf | thermal | dotmatrix | html
	Paper  *PrintPaper `yaml:"paper,omitempty" json:"paper,omitempty"`
}

// PrintPaper declares the paper size for a print output.
type PrintPaper struct {
	Size   string            `yaml:"size,omitempty" json:"size,omitempty"` // A4 | A5 | thermal_58mm | ...
	Margin string            `yaml:"margin,omitempty" json:"margin,omitempty"`
	Custom *PrintCustomPaper `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// PrintCustomPaper is a custom paper dimension.
type PrintCustomPaper struct {
	Width  float64 `yaml:"width" json:"width"`
	Height float64 `yaml:"height" json:"height"`
	Unit   string  `yaml:"unit" json:"unit"`
}

// PrintHeader is the declarative header block of a Print document.
type PrintHeader struct {
	Logo     bool   `yaml:"logo,omitempty" json:"logo,omitempty"`
	Title    string `yaml:"title,omitempty" json:"title,omitempty"`
	Subtitle string `yaml:"subtitle,omitempty" json:"subtitle,omitempty"`
}

// PrintBodyItem is one element in the Print body (fields | separator | child_table | totals).
type PrintBodyItem struct {
	Fields     []string         `yaml:"fields,omitempty" json:"fields,omitempty"`
	Separator  string           `yaml:"separator,omitempty" json:"separator,omitempty"`
	ChildTable *PrintChildTable `yaml:"child_table,omitempty" json:"child_table,omitempty"`
	Totals     *PrintTotals     `yaml:"totals,omitempty" json:"totals,omitempty"`
}

// PrintChildTable renders a child-table field in a Print body.
type PrintChildTable struct {
	Field   string   `yaml:"field" json:"field"`
	Columns []string `yaml:"columns" json:"columns"`
}

// PrintTotals renders a totals line in a Print body.
type PrintTotals struct {
	Field  string `yaml:"field" json:"field"`
	Format string `yaml:"format,omitempty" json:"format,omitempty"`
}

// PrintFooter is the declarative footer block of a Print document.
type PrintFooter struct {
	Text string `yaml:"text,omitempty" json:"text,omitempty"`
}

// TimelineSpec defines a chronological event journal (Frontend §13).
type TimelineSpec struct {
	// @schema {description: "If true (default), a route /module/timeline/<name> is auto-generated.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "clinic.medical_record"}
	Entity     string           `yaml:"entity" json:"entity"`
	EventField string           `yaml:"event_field,omitempty" json:"event_field,omitempty"`
	DateField  string           `yaml:"date_field,omitempty" json:"date_field,omitempty"`
	BindParam  string           `yaml:"bind_param,omitempty" json:"bind_param,omitempty"`
	BindValue  string           `yaml:"bind_value,omitempty" json:"bind_value,omitempty"`
	Display    *TimelineDisplay `yaml:"display,omitempty" json:"display,omitempty"`
	GroupBy    string           `yaml:"group_by,omitempty" json:"group_by,omitempty"` // date | month | year | none
	Sort       string           `yaml:"sort,omitempty" json:"sort,omitempty"`         // asc | desc (default desc)
	PageSize   int              `yaml:"page_size,omitempty" json:"page_size,omitempty"`
	EmptyState string           `yaml:"empty_state,omitempty" json:"empty_state,omitempty"`
}

// TimelineDisplay maps entity fields to timeline card slots.
type TimelineDisplay struct {
	TitleField    string `yaml:"title_field,omitempty" json:"title_field,omitempty"`
	SubtitleField string `yaml:"subtitle_field,omitempty" json:"subtitle_field,omitempty"`
	ContentField  string `yaml:"content_field,omitempty" json:"content_field,omitempty"`
	IconField     string `yaml:"icon_field,omitempty" json:"icon_field,omitempty"`
	Component     string `yaml:"component,omitempty" json:"component,omitempty"`
}

// ThemeSpec defines look & feel as a distributable artifact (Frontend §10).
type ThemeSpec struct {
	// @schema {description: "If true (default), the theme is active and published in the bundle."}
	Public     *bool             `yaml:"public,omitempty" json:"public,omitempty"`
	Tokens     map[string]string `yaml:"tokens,omitempty" json:"tokens,omitempty"`
	Stylesheet string            `yaml:"stylesheet,omitempty" json:"stylesheet,omitempty"`
	Widgets    map[string]string `yaml:"widgets,omitempty" json:"widgets,omitempty"` // base widget → asset skin
}

// ─── 1.3 Missing Frontend Kind Structs ───

// CalendarSpec defines a calendar view for date/datetime entity data
// (06-page-kinds.md §5). Instance of VisualSpecKind tier: page.
type CalendarSpec struct {
	// @schema {description: "If true (default), a route /module/calendar/<name> is auto-generated.", example: "true"}
	Public *bool `yaml:"public,omitempty" json:"public,omitempty"`
	// @schema {example: "clinic.appointment"}
	Entity string `yaml:"entity" json:"entity"`
	// @schema {example: "scheduled_at"}
	DateField     string   `yaml:"date_field" json:"date_field"`
	EndField      string   `yaml:"end_field,omitempty" json:"end_field,omitempty"`
	TitleField    string   `yaml:"title_field,omitempty" json:"title_field,omitempty"`
	ResourceField string   `yaml:"resource_field,omitempty" json:"resource_field,omitempty"`
	ColorField    string   `yaml:"color_field,omitempty" json:"color_field,omitempty"`
	Views         []string `yaml:"views,omitempty" json:"views,omitempty"` // month, week, day, resource (default month)
	Realtime      bool     `yaml:"realtime,omitempty" json:"realtime,omitempty"`
}

// ApprovalInboxSpec defines the pending-approval task queue page
// (06-page-kinds.md §11). Instance of VisualSpecKind tier: page.
// Zero-config: sources are pending Workflow steps eligible for the caller.
type ApprovalInboxSpec struct {
	Realtime bool         `yaml:"realtime,omitempty" json:"realtime,omitempty"`
	Filters  []FilterSpec `yaml:"filters,omitempty" json:"filters,omitempty"`
	Search   bool         `yaml:"search,omitempty" json:"search,omitempty"`
}

// NotificationCenterSpec defines the in-app notification page
// (06-page-kinds.md §12). Instance of VisualSpecKind tier: page.
// Zero-config: lists caller's notifications from formspec/notify.
type NotificationCenterSpec struct {
	Realtime bool `yaml:"realtime,omitempty" json:"realtime,omitempty"`
}

// ListingSpec defines a public catalog page (06-page-kinds.md §10).
// Instance of VisualSpecKind tier: page. Similar to table-list but without
// Auth-wrap assumptions and without row_actions/bulk_actions.
type ListingSpec struct {
	// @schema {example: "shop.product"}
	Entity  string        `yaml:"entity" json:"entity"`
	Columns []TableColumn `yaml:"columns" json:"columns"`
	Filters []FilterSpec  `yaml:"filters,omitempty" json:"filters,omitempty"`
	Search  bool          `yaml:"search,omitempty" json:"search,omitempty"`
}

// SectionBlockTypes is the closed set of section block types
// (frontend/06-page-kinds.md §1).
var SectionBlockTypes = map[string]bool{
	"hero":         true,
	"feature_grid": true,
	"card":         true,
	"carousel":     true,
	"cta":          true,
}

// ValidatePageSpec validates a PageSpec, returning an error if any constraint
// is violated. Enforces:
//   - `blocks` and `tabs` are mutually exclusive.
//   - Section blocks use a known type (closed set).
//   - `mode: custom` requires `asset` and forbids `blocks`/`tabs`.
func ValidatePageSpec(p *PageSpec) error {
	if len(p.Blocks) > 0 && len(p.Tabs) > 0 {
		return fmt.Errorf("page: `blocks` and `tabs` are mutually exclusive")
	}
	if p.Mode == "custom" {
		if p.Asset == "" {
			return fmt.Errorf("page: `mode: custom` requires `asset` (module-relative asset path)")
		}
		if len(p.Blocks) > 0 || len(p.Tabs) > 0 {
			return fmt.Errorf("page: `mode: custom` cannot declare `blocks` or `tabs` — the page is fully owned by the asset")
		}
	}
	for i, blk := range p.Blocks {
		if blk.Section == nil {
			continue
		}
		if !SectionBlockTypes[blk.Section.Type] {
			return fmt.Errorf("page block %d: section.type %q is not a known section block (closed set: hero, feature_grid, card, carousel, cta)", i, blk.Section.Type)
		}
	}
	if err := ValidateContextDecls(p.Context); err != nil {
		return fmt.Errorf("page: %w", err)
	}
	return nil
}
