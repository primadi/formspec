// ─── Forma Manifest TypeScript Types ───
//
// Mirror of Go types from:
//   - pkg/spec/frontend.go — 12 UI kind specs
//   - pkg/spec/entity.go   — Document/Entity spec, Field, Action, StateMachine
//   - pkg/spec/spec.go     — Enums, constants
//   - internal/ui/meta.go  — Meta API payloads
//
// Written manually once. Codegen (Fase 6.4) will replace this later.
// Field names use snake_case to match the JSON wire format from the Go server.



// ══════════════════════════════════════════════════════════════════════════════
// Constants
// ══════════════════════════════════════════════════════════════════════════════

export const API_VERSION = "forma.dev/v1alpha1"

// ── Resource Kinds ──

export const KIND_APP = "App"
export const KIND_MODULE = "Module"
export const KIND_DOCUMENT = "Document"
export const KIND_ENTITY = "Entity"
export const KIND_SERVICE = "Service"
export const KIND_CONFIG = "Config"
export const KIND_MIGRATION = "Migration"
export const KIND_SUBSCRIPTION = "Subscription"
export const KIND_WORKFLOW = "Workflow"
export const KIND_API = "Api"
export const KIND_WEBHOOK = "Webhook"
export const KIND_ENVIRONMENT = "Environment"
export const KIND_POLICY = "Policy"
export const KIND_DATASTORE = "Datastore"
export const KIND_PAGE = "Page"
export const KIND_FORM = "Form"
export const KIND_TABLE = "Table"
export const KIND_DASHBOARD = "Dashboard"
export const KIND_WIDGET = "Widget"
export const KIND_REPORT = "Report"
export const KIND_WIZARD = "Wizard"
export const KIND_KANBAN = "Kanban"
export const KIND_TIMELINE = "Timeline"
export const KIND_MENU = "Menu"
export const KIND_PRINT = "Print"
export const KIND_THEME = "Theme"

export type ResourceKind =
  | "App" | "Module" | "Document" | "Entity" | "Service" | "Config"
  | "Migration" | "Subscription" | "Workflow" | "Api" | "Webhook"
  | "Environment" | "Policy" | "Datastore"
  | "Page" | "Form" | "Table" | "Dashboard" | "Widget" | "Report"
  | "Wizard" | "Kanban" | "Timeline" | "Menu" | "Print" | "Theme"

// ── Field Types ──

export const FIELD_STRING = "string"
export const FIELD_INTEGER = "integer"
export const FIELD_DECIMAL = "decimal"
export const FIELD_BOOLEAN = "boolean"
export const FIELD_ENUM = "enum"
export const FIELD_DATE = "date"
export const FIELD_DATETIME = "datetime"
export const FIELD_JSON = "json"
export const FIELD_UUID = "uuid"
export const FIELD_RELATION = "relation"
export const FIELD_CHILD = "child"
export const FIELD_NUMBER = "number" // deprecated, backward compat

export type FieldType =
  | "string" | "integer" | "decimal" | "boolean" | "enum"
  | "date" | "datetime" | "json" | "uuid" | "relation" | "child" | "number"

// ── Characteristics ──

export const CHAR_MASTER = "master"
export const CHAR_TRANSACTION = "transaction"
export const CHAR_REFERENCE = "reference"
export const CHAR_SUMMARY = "summary"

export type Characteristic = "master" | "transaction" | "reference" | "summary"

// ── Lifecycle ──

export const LIFECYCLE_PLAIN_CRUD = "plain_crud"
export const LIFECYCLE_TWO_STEP_AUTOSAVE = "two_step_autosave"

export type Lifecycle = "plain_crud" | "two_step_autosave"

// ── Form Render Modes ──

export const RENDER_MODAL = "modal"
export const RENDER_DRAWER = "drawer"
export const RENDER_SEPARATE_PAGE = "separate_page"

export type FormRenderMode = "modal" | "drawer" | "separate_page"

// ── Impl Types ──

export const IMPL_SCRIPT_REF = "script_ref"
export const IMPL_SCRIPT = "script"
export const IMPL_NATIVE = "native"
export const IMPL_COMPILED = "compiled"
export const IMPL_SIDECAR = "sidecar"

export type ImplType = "script_ref" | "script" | "native" | "compiled" | "sidecar"

// ── Protocol Types ──

export const PROTOCOL_REST = "rest"
export const PROTOCOL_GRPC = "grpc"
export const PROTOCOL_WS = "ws"

export type ProtocolType = "rest" | "grpc" | "ws"

// ── Doc Status ──

export const DOC_STATUS_DRAFT = "draft"
export const DOC_STATUS_SUBMITTED = "submitted"
export const DOC_STATUS_CANCELLED = "cancelled"

export type DocStatus = "draft" | "submitted" | "cancelled" | ""

// ── Reserved Field Names ──

export const RESERVED_FIELD_NAMES = [
  "owner", "created_at", "modified", "doc_status",
  "amends", "amended_by", "version",
] as const

// ── Reserved Action Names ──

export const RESERVED_ACTION_NAMES = [
  "create", "update", "submit", "cancel", "delete",
  "amend", "create-submit", "amend-submit",
] as const

// ── Widget Types ──

export const WIDGET_METRIC = "metric"
export const WIDGET_CHART = "chart"
export const WIDGET_TABLE = "table"
export const WIDGET_LIST = "list"

export type WidgetType = "metric" | "chart" | "table" | "list"

// ── Filter Types ──

export const FILTER_TEXT = "text"
export const FILTER_SELECT = "select"
export const FILTER_DATE_RANGE = "date_range"

export type FilterType = "text" | "select" | "date_range"

// ── Export Formats ──

export const EXPORT_PDF = "pdf"
export const EXPORT_CSV = "csv"
export const EXPORT_XLSX = "xlsx"

export type ExportFormat = "pdf" | "csv" | "xlsx"

// ── Print Formats ──

export const PRINT_HTML = "html"
export const PRINT_PDF = "pdf"
export const PRINT_THERMAL = "thermal"
export const PRINT_DOTMATRIX = "dotmatrix"

export type PrintFormat = "pdf" | "thermal" | "dotmatrix" | "html"

// ══════════════════════════════════════════════════════════════════════════════
// Metadata & Raw Manifest
// ══════════════════════════════════════════════════════════════════════════════

export interface Metadata {
  name: string
  module?: string
  description?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

export interface RawManifest {
  apiVersion: string
  kind: string
  metadata: Metadata
  spec: Record<string, unknown>
}

// ══════════════════════════════════════════════════════════════════════════════
// Entity / Document Spec (from pkg/spec/entity.go)
// ══════════════════════════════════════════════════════════════════════════════

export interface DocumentSpec {
  version?: string
  plural?: string
  characteristic?: Characteristic
  auth?: EntityAuth
  persist?: PersistSpec
  fields: Field[]
  actions: Action[]
  state_machine?: StateMachine
  events?: EventDecl[]
  deliver?: DeliveryDecl[]
  indexes?: IndexDecl[]
  tenant?: TenantDecl
  extend_storage?: ExtendStorage
  expose?: ExposeConfig[]
  backdate_policy?: BackdatePolicy
  forward_date_policy?: ForwardDatePolicy
}

/** @deprecated Use DocumentSpec */
export type EntitySpec = DocumentSpec

export interface EntityAuth {
  required: boolean
  strategies: string[]
}

export interface PersistSpec {
  // TODO: add fields from the Go spec as needed
}

export interface Field {
  name: string
  type: FieldType
  title?: string
  description?: string
  default?: unknown
  required?: boolean
  immutable?: boolean
  unique?: boolean
  index?: boolean
  natural_key?: boolean
  natural_key_rule?: NaturalKeyRuleDecl
  audited?: boolean
  enum_values?: string[]
  rules?: ValidationRule[]
  relation?: RelationDecl
  child?: ChildDecl
  computed?: ComputedDecl
}

export interface ValidationRule {
  name: string
  value?: unknown
}

export interface FieldRef {
  entity: string
  field: string
}

export interface RelationDecl {
  type: "belongs_to" | "has_many" | "has_one"
  resource: string
  foreign_key?: string
  on_delete?: "restrict" | "cascade" | "set_null"
}

export interface ChildDecl {
  entity?: string
  storage: "jsonb" | "table"
  sequence_field?: string
  fields?: Field[]
}

export interface ComputedDecl {
  formula: string
}

export interface NaturalKeyRuleDecl {
  strategy: "sequence" | "custom"
  format?: string
  prefix?: NaturalKeyPrefix
  reset?: "never" | "yearly" | "monthly" | "daily"
}

export interface NaturalKeyPrefix {
  config?: string
  default?: string
  value?: string
}

// ── Actions ──

export interface Action {
  name: string
  description?: string
  required_permission?: string
  idempotent?: boolean
  idempotency_key?: IdempotencyDecl
  audit?: boolean
  disabled?: boolean
  call?: "sync" | "async"
  emits?: string
  expose?: string[]
  impl?: ImplDecl
  uses?: UsesDecl
  params?: ParamsDecl
  conditions?: ConditionDecl[]
  ui?: ActionUIHint
}

export interface ActionUIHint {
  button_label?: string
  style?: "primary" | "secondary" | "danger"
  icon?: string
  confirm?: string
  show_when?: string // FormaExpr
}

export interface IdempotencyDecl {
  from: string
  field?: string
}

export interface ImplDecl {
  type: ImplType
  ref?: string
}

export interface UsesDecl {
  resources?: string[]
  db?: UsesDbDecl
  config?: UsesConfigDecl
  kvstore?: KvstoreUseDecl[]
  primitives?: string[]
  datastores?: Record<string, string>
}

export interface UsesDbDecl {
  read?: string[]
  write?: string[]
}

export interface UsesConfigDecl {
  read?: string[]
  write?: string[]
}

export interface KvstoreUseDecl {
  scope?: "tenant" | "module" | "global"
  access?: "read" | "read_write"
  module?: string
}

export interface ParamsDecl {
  validate?: ParamValidation[]
}

export interface ParamValidation {
  field: string
  rules: ValidationRule[]
}

export interface ConditionDecl {
  script?: string
  message?: string
  field?: string
  expression?: string
}

// ── State Machine ──

export interface StateMachine {
  field: string
  initial: string
  states: StateDecl[]
  transitions: TransitionDecl[]
}

export interface StateDecl {
  name: string
  label: string
  description?: string
}

export interface TransitionDecl {
  from: string[]
  to: string
  /** The canonical key is `via`; legacy alias `action` also accepted */
  via: string
  guard?: GuardDecl
}

export interface GuardDecl {
  expression: string
  message?: string
}

// ── Events ──

export interface EventDecl {
  name: string
  description?: string
  type?: "sync" | "async"
  publish?: PublishDecl
  payload?: PayloadDecl
  deliver?: EventDeliveryDecl[]
}

export interface PublishDecl {
  durable?: boolean
}

export interface PayloadDecl {
  fields: string[]
}

export interface EventDeliveryDecl {
  channel: "audit_log" | "websocket" | "queue" | "reliable_event"
  target?: DeliveryTarget
  job?: string
  retry?: RetryDecl
  dead_letter?: DeliveryTarget
  idempotency_key?: string
}

export interface DeliveryTarget {
  scope?: string
  resource?: string
  action?: string
}

export interface RetryDecl {
  max?: number
  backoff?: "exponential" | "linear" | "fixed"
  initial_delay_ms?: number
}

/** @deprecated Use nested event.deliver[] */
export interface DeliveryDecl {
  event: string
  reliable_event?: boolean
  websocket?: boolean
  channel?: string
}

// ── Indexes, Tenant, Extend ──

export interface IndexDecl {
  fields: string[]
  unique?: boolean
}

export interface TenantDecl {
  isolated: boolean
}

export interface ExtendStorage {
  target: string
  namespace: string
}

export interface ExposeConfig {
  type: ProtocolType
  enabled?: boolean
  actions?: string[]
}

export interface BackdatePolicy {
  max_days_back?: number
  override_permission?: string
}

export interface ForwardDatePolicy {
  max_days_forward?: number
  override_permission?: string
}

// ══════════════════════════════════════════════════════════════════════════════
// Frontend Kind Specs (from pkg/spec/frontend.go)
// ══════════════════════════════════════════════════════════════════════════════

// ── Page (Frontend §3) ──

export interface PageSpec {
  route: string
  title: string
  icon?: string
  description?: string
  permissions?: string[]
  blocks?: PageBlock[]
  tabs?: PageTab[]
  layout?: PageLayout
}

export interface PageLayout {
  columns?: number
}

export interface PageBlock {
  form?: BlockRef
  table?: BlockRef
  component?: BlockRef
  widget?: BlockRef
  html?: string
}

export interface PageTab {
  label: string
  form?: BlockRef
  table?: BlockRef
  component?: BlockRef
}

export interface BlockRef {
  ref?: string
  asset?: string
  entity?: string
  id?: string
  mode?: "view" | "edit"
  param?: Record<string, unknown>
  props?: Record<string, unknown>
}

// ── Form (Frontend §4) ──

export interface FormSpec {
  entity: string
  mode?: "create" | "edit" | "view"
  sections: FormSection[]
  actions?: FormAction[]
  submit?: FormSubmit
  render?: FormRender
}

export interface FormSection {
  title: string
  description?: string
  columns?: number
  visible_when?: string
  fields: FormField[]
}

export interface FormField {
  name: string
  label?: string
  placeholder?: string
  help?: string
  widget?: string
  read_only?: boolean
  readonly_when?: string
  required_when?: string
  visible_when?: string
  compute?: string
}

export interface FormAction {
  action: string
  label?: string
  style?: "primary" | "secondary" | "danger"
  confirm?: string
}

export interface FormSubmit {
  label?: string
  redirect?: string
  message?: string
}

export interface FormRender {
  mode: FormRenderMode
}

// ── Table (Frontend §5) ──

export interface TableSpec {
  entity: string
  columns: TableColumn[]
  default_sort?: string
  page_size?: number
  search?: boolean
  realtime?: boolean
  row_actions?: TableAction[]
  bulk_actions?: TableAction[]
  filters?: TableFilter[]
}

export interface TableColumn {
  field: string
  label?: string
  sortable?: boolean
  width?: string
  align?: "left" | "center" | "right"
  link?: string
  format?: string
  widget?: string
}

export interface TableAction {
  action: string
  label: string
  icon?: string
  confirm_msg?: string
}

export interface TableFilter {
  field: string
  label: string
  type: FilterType
}

// ── Dashboard (Frontend §6) ──

export interface DashboardSpec {
  title: string
  description?: string
  customizable?: boolean
  defaults?: string[]
  refresh?: number
  realtime?: boolean
  widgets: DashboardWidget[]
}

export interface DashboardWidget {
  ref: string
  layout: WidgetLayout
  config?: Record<string, unknown>
}

export interface WidgetLayout {
  x: number
  y: number
  w: number
  h: number
}

// ── Widget (Frontend §7) ──

export interface WidgetSpec {
  title: string
  type: WidgetType
  entity?: string
  query?: string
  refresh_secs?: number
  size?: WidgetLayout
  config?: Record<string, unknown>
}

// ── Report (Frontend §8) ──

export interface ReportSpec {
  title: string
  entity: string
  required_permission?: string
  parameters?: ReportParam[]
  columns: ReportColumn[]
  groups?: string[]
  totals?: ReportTotal[]
  export?: ExportFormat[]
}

export interface ReportParam {
  name: string
  label: string
  type: string
  required?: boolean
  default?: unknown
}

export interface ReportColumn {
  field: string
  label: string
  aggregate?: string
  format?: string
}

export interface ReportTotal {
  label: string
  field: string
  fn: "sum" | "avg" | "count" | "min" | "max"
}

// ── Wizard (Frontend §11) ──

export interface WizardSpec {
  title: string
  entity?: string
  action?: string
  on_complete?: WizardOnComplete
  steps: WizardStep[]
}

export interface WizardOnComplete {
  restart?: boolean
  redirect?: string | null
  banner?: WizardSummaryItem[]
}

export interface WizardStep {
  title: string
  description?: string
  form?: string
  on_enter?: string
  on_next?: string
  on_prev?: string
  required?: string[]
  depends_on?: string
  entity?: string
  layout?: string
  search_fields?: string[]
  allow_create?: boolean
  fields?: FormField[]
  summary?: WizardSummaryItem[]
  component?: string
}

export interface WizardSummaryItem {
  label: string
  field: string
}

// ── Kanban (Frontend §12) ──

export interface KanbanSpec {
  entity: string
  status_field: string
  columns: KanbanColumn[]
  card_template?: KanbanCard
  realtime?: boolean
  filters?: string[]
  search?: boolean
  row_actions?: TableAction[]
  max_cards_per_column?: number
}

export interface KanbanColumn {
  status: string
  label: string
  color?: string
}

export interface KanbanCard {
  title: string
  subtitle?: string
  badge?: string
  assignee?: string
  fields?: string[]
  component?: string
}

// ── Menu (Frontend §9) ──

export interface MenuSpec {
  label: string
  icon?: string
  route?: string
  when?: string
  children?: MenuSpec[]
  order?: number
}

// ── Print (Frontend §9) ──

export interface PrintSpec {
  entity: string
  template?: string
  formats?: PrintFormat[]
  output?: PrintOutput
  header?: PrintHeader
  body?: PrintBodyItem[]
  footer?: PrintFooter
}

export interface PrintOutput {
  format: PrintFormat
  paper?: PrintPaper
}

export interface PrintPaper {
  size?: string
  margin?: string
  custom?: PrintCustomPaper
}

export interface PrintCustomPaper {
  width: number
  height: number
  unit: string
}

export interface PrintHeader {
  logo?: boolean
  title?: string
  subtitle?: string
}

export interface PrintBodyItem {
  fields?: string[]
  separator?: string
  child_table?: PrintChildTable
  totals?: PrintTotals
}

export interface PrintChildTable {
  field: string
  columns: string[]
}

export interface PrintTotals {
  field: string
  format?: string
}

export interface PrintFooter {
  text?: string
}

// ── Timeline (Frontend §13) ──

export interface TimelineSpec {
  entity: string
  event_field?: string
  date_field?: string
  bind_param?: string
  bind_value?: string
  display?: TimelineDisplay
  group_by?: "date" | "month" | "year" | "none"
  sort?: "asc" | "desc"
  page_size?: number
  empty_state?: string
}

export interface TimelineDisplay {
  title_field?: string
  subtitle_field?: string
  content_field?: string
  icon_field?: string
  component?: string
}

// ── Theme (Frontend §10) ──

export interface ThemeSpec {
  tokens?: Record<string, string>
  stylesheet?: string
  widgets?: Record<string, string>
}

// ══════════════════════════════════════════════════════════════════════════════
// Generic Entry wrapper (used in Meta API bundle)
// ══════════════════════════════════════════════════════════════════════════════

export interface Entry<T> {
  name: string
  module: string
  description?: string
  spec: T
}

// ══════════════════════════════════════════════════════════════════════════════
// Meta API Payloads (from internal/ui/meta.go)
// ══════════════════════════════════════════════════════════════════════════════

export interface EntitySchema {
  module: string
  name: string
  plural: string
  description?: string
  characteristic?: Characteristic
  label_field: string
  fields: Field[]
  state_machine?: StateMachine
  actions: ActionSummary[]
  lifecycle: Lifecycle
  has_quick_submit?: boolean
  exposed?: boolean
}

export interface ActionSummary {
  name: string
  description?: string
  permission: string
  has_params?: boolean
  ui?: ActionUIHint
}

export interface MetaBundle {
  entities: EntitySchema[]
  pages: Entry<PageSpec>[]
  forms: Entry<FormSpec>[]
  tables: Entry<TableSpec>[]
  dashboards: Entry<DashboardSpec>[]
  widgets: Entry<WidgetSpec>[]
  reports: Entry<ReportSpec>[]
  wizards: Entry<WizardSpec>[]
  kanbans: Entry<KanbanSpec>[]
  timelines: Entry<TimelineSpec>[]
  menus: Entry<MenuSpec>[]
  prints: Entry<PrintSpec>[]
  themes: Entry<ThemeSpec>[]
}

export interface MeResponse {
  user_id: string
  workspace: string
  roles: string[]
  permissions: string[]
}

// ══════════════════════════════════════════════════════════════════════════════
// API Response Envelopes
// ══════════════════════════════════════════════════════════════════════════════

export interface SingleResponse<T> {
  data: T
  meta?: ResponseMeta
}

export interface ListResponse<T> {
  data: T[]
  meta?: ListResponseMeta
}

export interface ResponseMeta {
  // empty placeholder for future use
}

export interface ListResponseMeta {
  page: number
  per_page: number
  total: number
  total_pages: number
}

export interface ErrorResponse {
  error: {
    code: string
    message: string
    details?: ErrorDetail[]
    request_id?: string
  }
}

export interface ErrorDetail {
  field?: string
  code: string
  message: string
}

// ══════════════════════════════════════════════════════════════════════════════
// API Client Types
// ══════════════════════════════════════════════════════════════════════════════

export interface ListParams {
  page?: number
  per_page?: number
  search?: string
  sort?: string // e.g. "-created_at" or "name"
  filters?: Record<string, string | FilterOpValue>
}

export interface FilterOpValue {
  op: FilterOp
  value: string
}

export type FilterOp = "eq" | "neq" | "gt" | "gte" | "lt" | "lte" | "like" | "in" | "nin"

export class FormaApiError extends Error {
  status: number
  code: string
  details?: ErrorDetail[]

  constructor(
    status: number,
    code: string,
    message: string,
    details?: ErrorDetail[],
  ) {
    super(message)
    this.name = "FormaApiError"
    this.status = status
    this.code = code
    this.details = details
  }
}

// ══════════════════════════════════════════════════════════════════════════════
// Derived / Computed Types
// ══════════════════════════════════════════════════════════════════════════════

/**
 * The three lifecycle patterns derived server-side (Frontend §1.7).
 * Sent as the `lifecycle` field on EntitySchema.
 */
export type LifecyclePattern = "plain_crud" | "two_step_autosave"

/**
 * Heuristic form render mode derived by the derivation engine.
 * Matches FormRender.mode values.
 */
export type DeriveFormRenderMode = "modal" | "drawer" | "separate_page"

/**
 * Map from entity reference ("module/name") to EntitySchema, for quick lookup.
 */
export type EntityMap = Map<string, EntitySchema>
