// Package spec — Error Glossary Go Types
//
// Maps the canonical error codes from docs/spec/backend/error-glossary.yaml
// into Go constants and a structured error type.
//
// Generated from error-glossary.yaml v1.0.0.
//
// Error codes follow the pattern FORMA.{DOMAIN}.{NAME}. New codes MUST NOT
// reuse an existing code — third-party integrations that switch(error.code)
// must not silently break.

package spec

// ErrorCode is a canonical Forma error code string.
type ErrorCode string

// String returns the error code as a string.
func (c ErrorCode) String() string { return string(c) }

// ─── 1.5.1 Document Lifecycle (FORMA.DOC.*) ───

const (
	ErrorDocUpdateNotDraft       ErrorCode = "FORMA.DOC.UPDATE_NOT_DRAFT"
	ErrorDocDeleteReferenced     ErrorCode = "FORMA.DOC.DELETE_REFERENCED"
	ErrorDocCancelReferenced     ErrorCode = "FORMA.DOC.CANCEL_REFERENCED"
	ErrorDocSubmitNotDraft       ErrorCode = "FORMA.DOC.SUBMIT_NOT_DRAFT"
	ErrorDocAlreadySubmitted     ErrorCode = "FORMA.DOC.ALREADY_SUBMITTED"
	ErrorDocAlreadyCancelled     ErrorCode = "FORMA.DOC.ALREADY_CANCELLED"
	ErrorDocReservedField        ErrorCode = "FORMA.DOC.RESERVED_FIELD"
	ErrorDocCreateSubmitNotAvail ErrorCode = "FORMA.DOC.CREATE_SUBMIT_NOT_AVAILABLE"
)

// ─── Transaction Date (FORMA.TXN.*) ───

const (
	ErrorTxnTransactionDateMissing ErrorCode = "FORMA.TXN.TRANSACTION_DATE_MISSING"
	ErrorTxnBackdateExceeded       ErrorCode = "FORMA.TXN.BACKDATE_EXCEEDED"
	ErrorTxnForwardDateExceeded    ErrorCode = "FORMA.TXN.FORWARD_DATE_EXCEEDED"
)

// ─── Period Closing (FORMA.PERIOD.*) ───

const (
	ErrorPeriodClosed       ErrorCode = "FORMA.PERIOD.CLOSED"
	ErrorPeriodReopenDenied ErrorCode = "FORMA.PERIOD.REOPEN_DENIED"
)

// ─── Event Naming Convention (FORMA.EVENT.*) ───

const (
	ErrorEventTypeMismatch ErrorCode = "FORMA.EVENT.TYPE_MISMATCH"
	ErrorEventTypeMissing  ErrorCode = "FORMA.EVENT.TYPE_MISSING"
)

// ─── Saga & Manual Intervention (FORMA.SAGA.*) ───

const (
	ErrorSagaOutcomeUnknown     ErrorCode = "FORMA.SAGA.OUTCOME_UNKNOWN"
	ErrorSagaCompensationFailed ErrorCode = "FORMA.SAGA.COMPENSATION_FAILED"
)

// ─── Reference Guard (FORMA.REF.*) ───

const (
	ErrorRefDeleteBlocked ErrorCode = "FORMA.REF.DELETE_BLOCKED"
	ErrorRefCancelBlocked ErrorCode = "FORMA.REF.CANCEL_BLOCKED"
)

// ─── Persistence & Category Isolation (FORMA.PERSIST.*) ───

const (
	ErrorPersistCrossCategory ErrorCode = "FORMA.PERSIST.CROSS_CATEGORY"
)

// ─── Archiving (FORMA.ARCHIVE.*) ───

const (
	ErrorArchiveLockedForDeletion ErrorCode = "FORMA.ARCHIVE.LOCKED_FOR_DELETION"
)

// ─── General Validation (FORMA.VALIDATE.*) ───

const (
	ErrorValidateReservedActionModified ErrorCode = "FORMA.VALIDATE.RESERVED_ACTION_MODIFIED"
)

// ─── 1.5.2 Observability Error Codes (09-observability.md §8) ───

const (
	ErrorObservabilityMetricsDisabled ErrorCode = "OBSERVABILITY_METRICS_DISABLED"
	ErrorObservabilityDebugForbidden  ErrorCode = "OBSERVABILITY_DEBUG_FORBIDDEN"
	ErrorLogsFilterInvalid            ErrorCode = "LOGS_FILTER_INVALID"
)

// ─── ErrorParams — structured error details ───

// ErrorParams carries the parameter values for an error code's message template.
// Each entry maps a param name (from error-glossary.yaml) to its value.
type ErrorParams map[string]string

// FormaError is a structured error with a canonical code, human message, and
// optional parameter details. Used in API error envelopes (01-core-basic.md §8.5).
type FormaError struct {
	Code    ErrorCode     `yaml:"code" json:"code"`
	Message string        `yaml:"message" json:"message"`
	Details []ErrorDetail `yaml:"details,omitempty" json:"details,omitempty"`
}

// ErrorDetail provides structured per-field or per-level error context.
type ErrorDetail struct {
	Level   string `yaml:"level,omitempty" json:"level,omitempty"` // validation level: L4, L5, L6
	Field   string `yaml:"field,omitempty" json:"field,omitempty"` // field name if field-level
	Message string `yaml:"message" json:"message"`                 // human-readable detail
}

// Error implements the error interface for FormaError.
func (e *FormaError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

// ErrorCodeSet returns all defined error codes.
func ErrorCodeSet() []ErrorCode {
	return []ErrorCode{
		// FORMA.DOC
		ErrorDocUpdateNotDraft, ErrorDocDeleteReferenced, ErrorDocCancelReferenced,
		ErrorDocSubmitNotDraft, ErrorDocAlreadySubmitted, ErrorDocAlreadyCancelled,
		ErrorDocReservedField, ErrorDocCreateSubmitNotAvail,
		// FORMA.TXN
		ErrorTxnTransactionDateMissing, ErrorTxnBackdateExceeded, ErrorTxnForwardDateExceeded,
		// FORMA.PERIOD
		ErrorPeriodClosed, ErrorPeriodReopenDenied,
		// FORMA.EVENT
		ErrorEventTypeMismatch, ErrorEventTypeMissing,
		// FORMA.SAGA
		ErrorSagaOutcomeUnknown, ErrorSagaCompensationFailed,
		// FORMA.REF
		ErrorRefDeleteBlocked, ErrorRefCancelBlocked,
		// FORMA.PERSIST
		ErrorPersistCrossCategory,
		// FORMA.ARCHIVE
		ErrorArchiveLockedForDeletion,
		// FORMA.VALIDATE
		ErrorValidateReservedActionModified,
		// Observability
		ErrorObservabilityMetricsDisabled, ErrorObservabilityDebugForbidden, ErrorLogsFilterInvalid,
	}
}
