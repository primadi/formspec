import type { ErrorDetailItem, ErrorResponse } from "./types.js";

/**
 * Thrown for any non-2xx response. Mirrors the error envelope exactly
 * (docs/spec/02-core-basic.md §16) so callers can branch on `code` instead
 * of parsing messages: VALIDATION_ERROR (422), UNAUTHORIZED (401),
 * FORBIDDEN (403), NOT_FOUND (404), CONFLICT / version conflict (409),
 * INTERNAL_ERROR (500), CONDITION_FAILED (422, action guard), ACTION_ERROR (500).
 */
export class FormaApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: ErrorDetailItem[];

  constructor(status: number, body: ErrorResponse | undefined, fallbackMessage: string) {
    super(body?.error.message ?? fallbackMessage);
    this.name = "FormaApiError";
    this.status = status;
    this.code = body?.error.code ?? "UNKNOWN_ERROR";
    this.details = body?.error.details;
  }

  /** True for a permission check failure (403). */
  get isForbidden(): boolean {
    return this.status === 403;
  }

  /** True when the record was concurrently modified (409) — refetch and retry. */
  get isConflict(): boolean {
    return this.status === 409;
  }

  /** True for field-level validation failures (422) — see `.details`. */
  get isValidation(): boolean {
    return this.status === 422;
  }
}
