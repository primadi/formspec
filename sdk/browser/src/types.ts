/**
 * Wire types — mirror the Go response envelopes in internal/api/handler.go
 * (docs/spec/02-core-basic.md §16). Every Forma entity record is the
 * entity's own fields flattened alongside these reserved framework columns
 * (internal/db/crud.go EntityRecord.MarshalJSON).
 */
export interface RecordMeta {
  id: string;
  tenant_id: string;
  version: number;
  created_at: string;
  updated_at: string;
  created_by: string;
  updated_by: string;
}

/** A Forma entity record: framework columns + the entity's declared fields. */
export type FormaRecord<T = Record<string, unknown>> = RecordMeta & T;

export interface MetaSingle {
  request_id?: string;
  timestamp: string;
}

export interface MetaList {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface ListLinks {
  first?: string;
  last?: string;
  next?: string;
  prev?: string;
}

export interface SingleResponse<T> {
  data: T;
  meta: MetaSingle;
}

export interface ListResponse<T> {
  data: T[];
  meta: MetaList;
  links: ListLinks;
}

export interface ListResult<T> {
  data: T[];
  page: number;
  perPage: number;
  total: number;
  totalPages: number;
}

export interface ErrorDetailItem {
  level?: string;
  field?: string;
  message: string;
}

export interface ErrorDetail {
  code: string;
  message: string;
  details?: ErrorDetailItem[];
}

export interface ErrorResponse {
  error: ErrorDetail;
  meta: MetaSingle;
}

/**
 * List query options. Only page/perPage/search are wired server-side today
 * (internal/api/handler.go HandleList parses page, per_page, search — sort
 * and filter[...] are declared in the spec (§16) but not yet implemented,
 * so they are intentionally absent here rather than silently ignored).
 */
export interface ListOptions {
  page?: number;
  perPage?: number;
  search?: string;
}
