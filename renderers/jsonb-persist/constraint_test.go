package db

import (
	"errors"
	"fmt"
	"testing"
)

// classifyConstraintError must recognize BOTH drivers' text shapes. The kafe dev
// database is SQLite, so nothing else in the suite would notice if the
// PostgreSQL branch rotted — and PostgreSQL is the production target.
func TestClassifyConstraintError_DriverShapes(t *testing.T) {
	cases := []struct {
		name   string
		msg    string
		want   string // expected Detail
		unique bool
	}{
		{
			name:   "sqlite single column",
			msg:    "insert row: constraint failed: UNIQUE constraint failed: cafe_order_table_sessions._dining_table_id (2067)",
			want:   "dining_table_id",
			unique: true,
		},
		{
			name:   "sqlite composite drops the tenant scope column",
			msg:    "constraint failed: UNIQUE constraint failed: cafe_master_dining_tables.tenant_id, cafe_master_dining_tables._branch_id, cafe_master_dining_tables._code",
			want:   "branch_id, code",
			unique: true,
		},
		{
			name:   "postgres constraint name",
			msg:    `pq: duplicate key value violates unique constraint "cafe_master_menu_item_prices_branch_id_menu_item_id_key"`,
			want:   "cafe_master_menu_item_prices_branch_id_menu_item_id_key",
			unique: true,
		},
		{
			name: "postgres DETAIL columns win over the constraint name",
			msg: `pq: duplicate key value violates unique constraint "cafe_order_table_sessions_dining_table_id_key"` +
				` DETAIL: Key (dining_table_id)=(01a0e3f0-...) already exists.`,
			want:   "dining_table_id",
			unique: true,
		},
		{
			name:   "postgres bare sqlstate",
			msg:    "mutation failed: SQLSTATE 23505",
			want:   "",
			unique: true,
		},
		{
			name:   "foreign key is NOT a uniqueness violation",
			msg:    "insert row: constraint failed: FOREIGN KEY constraint failed (787)",
			unique: false,
		},
		{
			name:   "not-null is NOT a uniqueness violation",
			msg:    "constraint failed: NOT NULL constraint failed: table.code",
			unique: false,
		},
		{
			name:   "unrelated error passes through",
			msg:    "database is locked",
			unique: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyConstraintError(fmt.Errorf("%s", tc.msg))
			if got := errors.Is(err, ErrUniqueViolation); got != tc.unique {
				t.Fatalf("errors.Is(ErrUniqueViolation) = %v, want %v (err=%v)", got, tc.unique, err)
			}
			if !tc.unique {
				// A non-match must be returned UNCHANGED — wrapping everything
				// would make every storage error look like a conflict.
				if err.Error() != tc.msg {
					t.Errorf("non-matching error must pass through unchanged, got %q", err.Error())
				}
				return
			}
			if tc.want != "" {
				var uv *UniqueViolationError
				if !errors.As(err, &uv) {
					t.Fatalf("expected a *UniqueViolationError, got %T", err)
				}
				if uv.Detail != tc.want {
					t.Errorf("Detail = %q, want %q", uv.Detail, tc.want)
				}
			}
		})
	}
}

// classifyConstraintError must be idempotent: an error that is already
// classified no longer carries the driver text, so re-classifying it must not
// wrap it a second time (a double wrap would still satisfy errors.Is, but it
// would nest needlessly and lose the outermost Detail).
func TestClassifyConstraintError_Idempotent(t *testing.T) {
	once := classifyConstraintError(fmt.Errorf("UNIQUE constraint failed: t._code"))
	twice := classifyConstraintError(once)

	var uv *UniqueViolationError
	if !errors.As(twice, &uv) {
		t.Fatalf("expected a *UniqueViolationError, got %T", twice)
	}
	if uv.Detail != "code" {
		t.Errorf("Detail = %q, want %q", uv.Detail, "code")
	}
	// The outer error must not have been replaced by a new wrapper.
	if twice != once {
		t.Errorf("second classification must be a no-op, got a new error")
	}
}

func TestClassifyConstraintError_NilIsNil(t *testing.T) {
	if err := classifyConstraintError(nil); err != nil {
		t.Fatalf("nil must stay nil, got %v", err)
	}
}
