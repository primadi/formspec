package spec

// ─── Frontend Kinds (Frontend Spec §2–§12) ───

// PageSpec defines a routed screen composing blocks (Frontend §3).
type PageSpec struct {
	Route       string      `yaml:"route" json:"route"`
	Title       string      `yaml:"title" json:"title"`
	Icon        string      `yaml:"icon,omitempty" json:"icon,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Permissions []string    `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Blocks      []PageBlock `yaml:"blocks" json:"blocks"`
}

// PageBlock is a compositional unit within a Page.
type PageBlock struct {
	Form      *BlockRef `yaml:"form,omitempty" json:"form,omitempty"`
	Table     *BlockRef `yaml:"table,omitempty" json:"table,omitempty"`
	Component *BlockRef `yaml:"component,omitempty" json:"component,omitempty"`
	Widget    *BlockRef `yaml:"widget,omitempty" json:"widget,omitempty"`
	HTML      string    `yaml:"html,omitempty" json:"html,omitempty"`
}

// BlockRef references another resource within a page block.
type BlockRef struct {
	Ref    string         `yaml:"ref" json:"ref"`
	Entity string         `yaml:"entity,omitempty" json:"entity,omitempty"`
	ID     string         `yaml:"id,omitempty" json:"id,omitempty"`
	Mode   string         `yaml:"mode,omitempty" json:"mode,omitempty"` // view | edit
	Param  map[string]any `yaml:"param,omitempty" json:"param,omitempty"`
}

// FormSpec defines an input/edit layout (Frontend §4).
type FormSpec struct {
	Entity   string        `yaml:"entity" json:"entity"`
	Sections []FormSection `yaml:"sections" json:"sections"`
	Submit   *FormSubmit   `yaml:"submit,omitempty" json:"submit,omitempty"`
	Render   *FormRender   `yaml:"render,omitempty" json:"render,omitempty"`
}

// FormSection groups form fields.
type FormSection struct {
	Title       string      `yaml:"title" json:"title"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Fields      []FormField `yaml:"fields" json:"fields"`
}

// FormField configures a single field in a form.
type FormField struct {
	Name         string `yaml:"name" json:"name"`
	Label        string `yaml:"label,omitempty" json:"label,omitempty"`
	Placeholder  string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Help         string `yaml:"help,omitempty" json:"help,omitempty"`
	ReadOnly     bool   `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	RequiredWhen string `yaml:"required_when,omitempty" json:"required_when,omitempty"`
	VisibleWhen  string `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
}

// FormSubmit configures the submit button and post-submit behavior.
type FormSubmit struct {
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Redirect string `yaml:"redirect,omitempty" json:"redirect,omitempty"`
	Message  string `yaml:"message,omitempty" json:"message,omitempty"`
}

// FormRender controls how a form is displayed (modal, drawer, page).
type FormRender struct {
	Mode string `yaml:"mode" json:"mode"` // modal | drawer | separate_page
}

// TableSpec defines a list/browse view (Frontend §5).
type TableSpec struct {
	Entity      string        `yaml:"entity" json:"entity"`
	Columns     []TableColumn `yaml:"columns" json:"columns"`
	DefaultSort string        `yaml:"default_sort,omitempty" json:"default_sort,omitempty"`
	PageSize    int           `yaml:"page_size,omitempty" json:"page_size,omitempty"`
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
	Align    string `yaml:"align,omitempty" json:"align,omitempty"` // left | center | right
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

// DashboardSpec defines a widget canvas (Frontend §6).
type DashboardSpec struct {
	Title       string            `yaml:"title" json:"title"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Widgets     []DashboardWidget `yaml:"widgets" json:"widgets"`
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

// WidgetSpec defines a reusable dashboard widget (Frontend §6).
type WidgetSpec struct {
	Title       string         `yaml:"title" json:"title"`
	Type        string         `yaml:"type" json:"type"` // metric | chart | table | list
	Entity      string         `yaml:"entity,omitempty" json:"entity,omitempty"`
	Query       string         `yaml:"query,omitempty" json:"query,omitempty"`
	RefreshSecs int            `yaml:"refresh_secs,omitempty" json:"refresh_secs,omitempty"`
	Config      map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

// ReportSpec defines a parameterized tabular report (Frontend §7).
type ReportSpec struct {
	Title      string         `yaml:"title" json:"title"`
	Entity     string         `yaml:"entity" json:"entity"`
	Parameters []ReportParam  `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Columns    []ReportColumn `yaml:"columns" json:"columns"`
	Groups     []string       `yaml:"groups,omitempty" json:"groups,omitempty"`
	Totals     []ReportTotal  `yaml:"totals,omitempty" json:"totals,omitempty"`
	Export     []string       `yaml:"export,omitempty" json:"export,omitempty"` // pdf | csv | xlsx
}

// ReportParam is a filterable input for a report.
type ReportParam struct {
	Name    string `yaml:"name" json:"name"`
	Label   string `yaml:"label" json:"label"`
	Type    string `yaml:"type" json:"type"`
	Default any    `yaml:"default,omitempty" json:"default,omitempty"`
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

// WizardSpec defines a multi-step business process (Frontend §8).
type WizardSpec struct {
	Title  string       `yaml:"title" json:"title"`
	Entity string       `yaml:"entity,omitempty" json:"entity,omitempty"`
	Steps  []WizardStep `yaml:"steps" json:"steps"`
	Save   string       `yaml:"save,omitempty" json:"save,omitempty"` // draft | partial | full
}

// WizardStep is one step in a multi-step wizard.
type WizardStep struct {
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Form        string `yaml:"form,omitempty" json:"form,omitempty"`
	Action      string `yaml:"action,omitempty" json:"action,omitempty"`
	DependsOn   string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

// KanbanSpec defines a drag-and-drop status board (Frontend §9).
type KanbanSpec struct {
	Entity       string         `yaml:"entity" json:"entity"`
	StatusField  string         `yaml:"status_field" json:"status_field"`
	Columns      []KanbanColumn `yaml:"columns" json:"columns"`
	CardTemplate *KanbanCard    `yaml:"card_template,omitempty" json:"card_template,omitempty"`
}

// KanbanColumn is a status lane in a kanban board.
type KanbanColumn struct {
	Status string `yaml:"status" json:"status"`
	Label  string `yaml:"label" json:"label"`
}

// KanbanCard defines what fields appear on a card.
type KanbanCard struct {
	Title  string   `yaml:"title" json:"title"`
	Fields []string `yaml:"fields" json:"fields"`
}

// MenuSpec defines a navigation entry (Frontend §11).
type MenuSpec struct {
	Label    string     `yaml:"label" json:"label"`
	Icon     string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Route    string     `yaml:"route,omitempty" json:"route,omitempty"`
	Children []MenuSpec `yaml:"children,omitempty" json:"children,omitempty"`
	Order    int        `yaml:"order,omitempty" json:"order,omitempty"`
}

// PrintSpec defines a printable document template (Frontend §12).
type PrintSpec struct {
	Entity   string   `yaml:"entity" json:"entity"`
	Template string   `yaml:"template" json:"template"`
	Formats  []string `yaml:"formats" json:"formats"` // pdf | thermal | dotmatrix | html
}

// TimelineSpec defines a chronological event journal (Frontend §10).
type TimelineSpec struct {
	Entity     string `yaml:"entity" json:"entity"`
	EventField string `yaml:"event_field" json:"event_field"`
	DateField  string `yaml:"date_field" json:"date_field"`
}
