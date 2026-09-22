package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func try(db *sql.DB, label, stmt string) {
	_, err := db.Exec(stmt)
	fmt.Printf("%-40s err=%v\n", label, err)
}

func show(db *sql.DB, label, query string) {
	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("  %s: query err=%v\n", label, err)
		return
	}
	defer func() { _ = rows.Close() }()
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		_ = rows.Scan(ptrs...)
		fmt.Printf("  %s: %v\n", label, vals)
	}
}

func main() {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		panic(err)
	}
	defer func() { _ = db.Close() }()

	// Old table: only branch_id was a derived column at creation (STORED),
	// cashier_id and status arrive later via ALTER.
	try(db, "create table", `CREATE TABLE shifts (
		id text primary key, tenant_id text, data text,
		_branch_id text GENERATED ALWAYS AS (json_extract(data, '$.branch_id')) STORED)`)
	try(db, "insert pre-existing row", `INSERT INTO shifts (id, tenant_id, data) VALUES ('1','kafe','{"branch_id":"A","cashier_id":"C1","status":"open"}')`)

	try(db, "ALTER cashier_id VIRTUAL", `ALTER TABLE shifts ADD COLUMN _cashier_id text GENERATED ALWAYS AS (json_extract(data, '$.cashier_id')) VIRTUAL`)
	try(db, "ALTER status VIRTUAL", `ALTER TABLE shifts ADD COLUMN _status text GENERATED ALWAYS AS (json_extract(data, '$.status')) VIRTUAL`)
	show(db, "pre-existing row values", `SELECT _branch_id, _cashier_id, _status FROM shifts`)

	try(db, "partial unique index over virtual cols", `CREATE UNIQUE INDEX idx_open_shift ON shifts (_branch_id, _cashier_id) WHERE _status = 'open'`)
	show(db, "index sql", `SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_open_shift'`)

	// Existing duplicate check must now already see the pre-existing row.
	try(db, "second open shift (must fail)", `INSERT INTO shifts (id, tenant_id, data) VALUES ('2','kafe','{"branch_id":"A","cashier_id":"C1","status":"open"}')`)
	try(db, "different cashier (must pass)", `INSERT INTO shifts (id, tenant_id, data) VALUES ('3','kafe','{"branch_id":"A","cashier_id":"C2","status":"open"}')`)
	try(db, "closed dup (must pass)", `INSERT INTO shifts (id, tenant_id, data) VALUES ('4','kafe','{"branch_id":"A","cashier_id":"C1","status":"closed"}')`)

	fmt.Println("--- money derived column as VIRTUAL")
	try(db, "create money table", `CREATE TABLE pays (id text primary key, data text)`)
	try(db, "insert money", `INSERT INTO pays (id, data) VALUES ('1','{"amount":{"amount":"75000","currency":"IDR"}}')`)
	try(db, "ALTER money VIRTUAL", `ALTER TABLE pays ADD COLUMN _amount numeric GENERATED ALWAYS AS (CAST(json_extract(data, '$.amount.amount') AS REAL)) VIRTUAL`)
	show(db, "money value", `SELECT _amount FROM pays`)
	try(db, "filter on money col", `SELECT id FROM pays WHERE _amount > 1`)
}
