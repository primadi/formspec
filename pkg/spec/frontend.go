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
type BlockRef struct {
	Ref    string         `yaml:"ref,omitempty" json:"ref,omitempty"`
	Asset  string         `yaml:"asset,omitempty" json:"asset,omitempty"` // component blocks (§7)
	Entity string         `yaml:"entity,omitempty" json:"entity,omitempty"`
	ID     string         `yaml:"id,omitempty" json:"id,omitempty"`
	Mode   string         `yaml:"mode,omitempty" json:"mode,omitempty"` // view | edit
	Param  map[string]any `yaml:"param,omitempty" json:"param,omitempty"`
	Props  map[string]any `yaml:"props,omitempty" json:"props,omitempty"` // component blocks
}

// FormSpec defines an input/edit layout (Frontend §4).
type FormSpec struct {
	Entity   string        `yaml:"entity" json:"entity"`
	Mode     string        `yaml:"mode,omitempty" json:"mode,omitempty"` // create | edit | view
	Sections []FormSection `yaml:"sections" json:"sections"`
	Actions  []FormAction  `yaml:"actions,omitempty" json:"actions,omitempty"`
	Submit   *FormSubmit   `yaml:"submit,omitempty" json:"submit,omitempty"`
	Render   *FormRender   `yaml:"render,omitempty" json:"render,omitempty"`
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
type FormField struct {
	Name         string `yaml:"name" json:"name"`
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
type FormRender struct {
	Mode string `yaml:"mode" json:"mode"` // modal | drawer | separate_page
}

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
	Groups             []string       `yaml:"groups,omitempty" json:"groups,omitempty"`
	Totals             []ReportTotal  `yaml:"totals,omitempty" json:"totals,omitempty"`
	Export             []string       `yaml:"export,omitempty" json:"export,omitempty"` // pdf | csv | xlsx
}

// ReportParam is a filterable input for a report.
type ReportParam struct {
	Name     string `yaml:"name" json:"name"`
	Label    string `yaml:"label" json:"label"`
	Type     string `yaml:"type" json:"type"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default  any    `yaml:"default,omitempty" json:"default,omitempty"`
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
	Title            string       `yaml:"title" json:"title"`
	Entity           string       `yaml:"entity,omitempty" json:"entity,omitempty"`
	Action           string       `yaml:"action,omitempty" json:"action,omitempty"` // server action that commits all steps
	AllowPartialSave bool         `yaml:"allow_partial_save,omitempty" json:"allow_partial_save,omitempty"`
	Steps            []WizardStep `yaml:"steps" json:"steps"`
	Save             string       `yaml:"save,omitempty" json:"save,omitempty"` // draft | partial | full (legacy alias of allow_partial_save)
}

// WizardStep is one step in a multi-step wizard.
type WizardStep struct {
	Title        string              `yaml:"title" json:"title"`
	Description  string              `yaml:"description,omitempty" json:"description,omitempty"`
	Form         string              `yaml:"form,omitempty" json:"form,omitempty"`
	Action       string              `yaml:"action,omitempty" json:"action,omitempty"`
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

// MenuSpec defines a navigation entry (Frontend §9).
type MenuSpec struct {
	Label    string     `yaml:"label" json:"label"`
	Icon     string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Route    string     `yaml:"route,omitempty" json:"route,omitempty"`
	When     string     `yaml:"when,omitempty" json:"when,omitempty"` // FormaExpr business condition
	Children []MenuSpec `yaml:"children,omitempty" json:"children,omitempty"`
	Order    int        `yaml:"order,omitempty" json:"order,omitempty"`
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
