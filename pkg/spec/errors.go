// Package spec — Error Glossary Go Types
//
// Maps the canonical error codes from docs/spec/backend/error-glossary.yaml
// into Go constants and a structured error type.
//
// Generated from error-glossary.yaml v1.0.0.
//
// Error codes follow the pattern FORMSPEC.{DOMAIN}.{NAME}. New codes MUST NOT
// reuse an existing code — third-party integrations that switch(error.code)
// must not silently break.

package spec

// ErrorCode is a canonical FormSpec error code string.
type ErrorCode string

// String returns the error code as a string.
func (c ErrorCode) String() string { return string(c) }

// ─── 1.5.1 Document Lifecycle (FORMSPEC.DOC.*) ───

const (
	ErrorDocUpdateNotDraft       ErrorCode = "FORMSPEC.DOC.UPDATE_NOT_DRAFT"
	ErrorDocDeleteReferenced     ErrorCode = "FORMSPEC.DOC.DELETE_REFERENCED"
	ErrorDocCancelReferenced     ErrorCode = "FORMSPEC.DOC.CANCEL_REFERENCED"
	ErrorDocSubmitNotDraft       ErrorCode = "FORMSPEC.DOC.SUBMIT_NOT_DRAFT"
	ErrorDocAlreadySubmitted     ErrorCode = "FORMSPEC.DOC.ALREADY_SUBMITTED"
	ErrorDocAlreadyCancelled     ErrorCode = "FORMSPEC.DOC.ALREADY_CANCELLED"
	ErrorDocReservedField        ErrorCode = "FORMSPEC.DOC.RESERVED_FIELD"
	ErrorDocCreateSubmitNotAvail ErrorCode = "FORMSPEC.DOC.CREATE_SUBMIT_NOT_AVAILABLE"
)

// ─── Transaction Date (FORMSPEC.TXN.*) ───

const (
	ErrorTxnTransactionDateMissing ErrorCode = "FORMSPEC.TXN.TRANSACTION_DATE_MISSING"
	ErrorTxnBackdateExceeded       ErrorCode = "FORMSPEC.TXN.BACKDATE_EXCEEDED"
	ErrorTxnForwardDateExceeded    ErrorCode = "FORMSPEC.TXN.FORWARD_DATE_EXCEEDED"
)

// ─── Period Closing (FORMSPEC.PERIOD.*) ───

const (
	ErrorPeriodClosed       ErrorCode = "FORMSPEC.PERIOD.CLOSED"
	ErrorPeriodReopenDenied ErrorCode = "FORMSPEC.PERIOD.REOPEN_DENIED"
)

// ─── Event Naming Convention (FORMSPEC.EVENT.*) ───

const (
	ErrorEventTypeMismatch ErrorCode = "FORMSPEC.EVENT.TYPE_MISMATCH"
	ErrorEventTypeMissing  ErrorCode = "FORMSPEC.EVENT.TYPE_MISSING"
)

// ─── Saga & Manual Intervention (FORMSPEC.SAGA.*) ───

const (
	ErrorSagaOutcomeUnknown     ErrorCode = "FORMSPEC.SAGA.OUTCOME_UNKNOWN"
	ErrorSagaCompensationFailed ErrorCode = "FORMSPEC.SAGA.COMPENSATION_FAILED"
)

// ─── Reference Guard (FORMSPEC.REF.*) ───

const (
	ErrorRefDeleteBlocked ErrorCode = "FORMSPEC.REF.DELETE_BLOCKED"
	ErrorRefCancelBlocked ErrorCode = "FORMSPEC.REF.CANCEL_BLOCKED"
)

// ─── Persistence & Category Isolation (FORMSPEC.PERSIST.*) ───

const (
	ErrorPersistCrossCategory ErrorCode = "FORMSPEC.PERSIST.CROSS_CATEGORY"
)

// ─── Archiving (FORMSPEC.ARCHIVE.*) ───

const (
	ErrorArchiveLockedForDeletion ErrorCode = "FORMSPEC.ARCHIVE.LOCKED_FOR_DELETION"
)

// ─── General Validation (FORMSPEC.VALIDATE.*) ───

const (
	ErrorValidateReservedActionModified ErrorCode = "FORMSPEC.VALIDATE.RESERVED_ACTION_MODIFIED"
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

// FormSpecError is a structured error with a canonical code, human message, and
// optional parameter details. Used in API error envelopes (01-core-basic.md §8.5).
type FormSpecError struct {
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

// Error implements the error interface for FormSpecError.
func (e *FormSpecError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

// ErrorCodeSet returns all defined error codes.
func ErrorCodeSet() []ErrorCode {
	return []ErrorCode{
		// FORMSPEC.DOC
		ErrorDocUpdateNotDraft, ErrorDocDeleteReferenced, ErrorDocCancelReferenced,
		ErrorDocSubmitNotDraft, ErrorDocAlreadySubmitted, ErrorDocAlreadyCancelled,
		ErrorDocReservedField, ErrorDocCreateSubmitNotAvail,
		// FORMSPEC.TXN
		ErrorTxnTransactionDateMissing, ErrorTxnBackdateExceeded, ErrorTxnForwardDateExceeded,
		// FORMSPEC.PERIOD
		ErrorPeriodClosed, ErrorPeriodReopenDenied,
		// FORMSPEC.EVENT
		ErrorEventTypeMismatch, ErrorEventTypeMissing,
		// FORMSPEC.SAGA
		ErrorSagaOutcomeUnknown, ErrorSagaCompensationFailed,
		// FORMSPEC.REF
		ErrorRefDeleteBlocked, ErrorRefCancelBlocked,
		// FORMSPEC.PERSIST
		ErrorPersistCrossCategory,
		// FORMSPEC.ARCHIVE
		ErrorArchiveLockedForDeletion,
		// FORMSPEC.VALIDATE
		ErrorValidateReservedActionModified,
		// Observability
		ErrorObservabilityMetricsDisabled, ErrorObservabilityDebugForbidden, ErrorLogsFilterInvalid,
	}
}
