package spec

// ─── Frontend Kinds (Frontend Spec §2–§13) ───

// PageSpec defines a routed screen composing blocks (Frontend §3).
// `blocks` and `tabs` are mutually exclusive.
type PageSpec struct {
	Route       string      `yaml:"route" json:"route"`
	Title       string      `yaml:"title" json:"title"`
	Icon        string      `yaml:"icon,omitempty" json:"icon,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Permissions []string    `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Blocks      []PageBlock `yaml:"blocks,omitempty" json:"blocks,omitempty"`
	Tabs        []PageTab   `yaml:"tabs,omitempty" json:"tabs,omitempty"`
	Layout      *PageLayout `yaml:"layout,omitempty" json:"layout,omitempty"`
}

// PageLayout controls block arrangement on a page.
type PageLayout struct {
	Columns int `yaml:"columns,omitempty" json:"columns,omitempty"`
}

// PageBlock is a compositional unit within a Page.
type PageBlock struct {
	Form      *BlockRef `yaml:"form,omitempty" json:"form,omitempty"`
	Table     *BlockRef `yaml:"table,omitempty" json:"table,omitempty"`
	Component *BlockRef `yaml:"component,omitempty" json:"component,omitempty"`
	Widget    *BlockRef `yaml:"widget,omitempty" json:"widget,omitempty"`
	HTML      string    `yaml:"html,omitempty" json:"html,omitempty"`
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
}

// FormSpec defines an input/edit layout (Frontend §4).
type FormSpec struct {
	Entity   string        `yaml:"entity" json:"entity"`
	Mode     string        `yaml:"mode,omitempty" json:"mode,omitempty"` // create | edit | view
	Sections []FormSection `yaml:"sections" json:"sections"`
	Actions  []FormAction  `yaml:"actions,omitempty" json:"actions,omitempty"`
	Submit   *FormSubmit   `yaml:"submit,omitempty" json:"submit,omitempty"`
	Render   FormRender    `yaml:"render,omitempty" json:"render,omitempty"`
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
type FormRender string // modal | drawer | separate_page

// TableSpec defines a list/browse view (Frontend §5).
type TableSpec struct {
	Entity      string        `yaml:"entity" json:"entity"`
	Columns     []TableColumn `yaml:"columns" json:"columns"`
	DefaultSort string        `yaml:"default_sort,omitempty" json:"default_sort,omitempty"`
	PageSize    int           `yaml:"page_size,omitempty" json:"page_size,omitempty"`
	Search      bool          `yaml:"search,omitempty" json:"search,omitempty"`
	Realtime    bool          `yaml:"realtime,omitempty" json:"realtime,omitempty"`
	RowActions  []TableAction `yaml:"row_actions,omitempty" json:"row_actions,omitempty"`
	BulkActions []TableAction `yaml:"bulk_actions,omitempty" json:"bulk_actions,omitempty"`
	Filters     []TableFilter `yaml:"filters,omitempty" json:"filters,omitempty"`
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

// TableFilter is a filterable field in a table.
type TableFilter struct {
	Field string `yaml:"field" json:"field"`
	Label string `yaml:"label" json:"label"`
	Type  string `yaml:"type" json:"type"` // text | select | date_range
}

// DashboardSpec defines a widget canvas (Frontend §5).
type DashboardSpec struct {
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
	Title       string         `yaml:"title" json:"title"`
	Type        string         `yaml:"type" json:"type"` // metric | chart | table | list
	Entity      string         `yaml:"entity,omitempty" json:"entity,omitempty"`
	Query       string         `yaml:"query,omitempty" json:"query,omitempty"`
	RefreshSecs int            `yaml:"refresh_secs,omitempty" json:"refresh_secs,omitempty"`
	Size        *WidgetLayout  `yaml:"size,omitempty" json:"size,omitempty"`
	Config      map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

// ReportSpec defines a parameterized tabular report (Frontend §5).
type ReportSpec struct {
	Title              string         `yaml:"title" json:"title"`
	Entity             string         `yaml:"entity" json:"entity"`
	RequiredPermission string         `yaml:"required_permission,omitempty" json:"required_permission,omitempty"`
	Parameters         []ReportParam  `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Columns            []ReportColumn `yaml:"columns" json:"columns"`
	Groups             []ReportGroup  `yaml:"groups,omitempty" json:"groups,omitempty"`
	Totals             []ReportTotal  `yaml:"totals,omitempty" json:"totals,omitempty"`
	Export             []string       `yaml:"export,omitempty" json:"export,omitempty"` // pdf | csv | xlsx
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
	Title      string            `yaml:"title" json:"title"`
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
	Entity            string         `yaml:"entity" json:"entity"`
	StatusField       string         `yaml:"status_field" json:"status_field"`
	Columns           []KanbanColumn `yaml:"columns" json:"columns"`
	CardTemplate      *KanbanCard    `yaml:"card_template,omitempty" json:"card_template,omitempty"`
	Realtime          *bool          `yaml:"realtime,omitempty" json:"realtime,omitempty"` // default true (§12)
	Filters           []string       `yaml:"filters,omitempty" json:"filters,omitempty"`
	Search            bool           `yaml:"search,omitempty" json:"search,omitempty"`
	RowActions        []TableAction  `yaml:"row_actions,omitempty" json:"row_actions,omitempty"`
	MaxCardsPerColumn int            `yaml:"max_cards_per_column,omitempty" json:"max_cards_per_column,omitempty"`
	Sortable          bool           `yaml:"sortable,omitempty" json:"sortable,omitempty"`             // enable within-column drag-to-reorder
	PositionField     string         `yaml:"position_field,omitempty" json:"position_field,omitempty"` // field storing user-adjustable position (e.g. "queue_position")
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
type PrintSpec struct {
	Entity   string          `yaml:"entity" json:"entity"`
	Template string          `yaml:"template,omitempty" json:"template,omitempty"`
	Formats  []string        `yaml:"formats,omitempty" json:"formats,omitempty"` // pdf | thermal | dotmatrix | html
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
	Tokens     map[string]string `yaml:"tokens,omitempty" json:"tokens,omitempty"`
	Stylesheet string            `yaml:"stylesheet,omitempty" json:"stylesheet,omitempty"`
	Widgets    map[string]string `yaml:"widgets,omitempty" json:"widgets,omitempty"` // base widget → asset skin
}

// ─── 1.3 Missing Frontend Kind Structs ───

// CalendarSpec defines a calendar view for date/datetime entity data
// (06-page-kinds.md §5). Instance of VisualSpecKind tier: page.
type CalendarSpec struct {
	Entity        string   `yaml:"entity" json:"entity"`
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
	Realtime bool          `yaml:"realtime,omitempty" json:"realtime,omitempty"`
	Filters  []TableFilter `yaml:"filters,omitempty" json:"filters,omitempty"`
	Search   bool          `yaml:"search,omitempty" json:"search,omitempty"`
}

// NotificationCenterSpec defines the in-app notification page
// (06-page-kinds.md §12). Instance of VisualSpecKind tier: page.
// Zero-config: lists caller's notifications from forma/notify.
type NotificationCenterSpec struct {
	Realtime bool `yaml:"realtime,omitempty" json:"realtime,omitempty"`
}

// ListingSpec defines a public catalog page (06-page-kinds.md §10).
// Instance of VisualSpecKind tier: page. Similar to table-list but without
// Auth-wrap assumptions and without row_actions/bulk_actions.
type ListingSpec struct {
	Entity  string        `yaml:"entity" json:"entity"`
	Columns []TableColumn `yaml:"columns" json:"columns"`
	Filters []TableFilter `yaml:"filters,omitempty" json:"filters,omitempty"`
	Search  bool          `yaml:"search,omitempty" json:"search,omitempty"`
}
