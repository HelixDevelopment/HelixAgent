package synchronization

// Regression guard for the GetCRDT nil-interface defect.
//
// DEFECT (pre-fix manager.go):
//
//	var crdt CRDT                     // nil interface
//	...
//	} else {                          // row-exists branch
//	    if err := crdt.FromJSON(stateJSON); err != nil {   // <-- nil-interface
//	                                                       //     method call
//
// Only the sql.ErrNoRows branch ever assigned `crdt`. The row-exists branch
// called a method on the still-nil interface, which is a hard runtime panic
// ("invalid memory address or nil pointer dereference"), not an error return.
//
// WHY IT NEVER FIRED: the `crdt_state` table did not exist in the deployed
// database, so every query failed with 42P01 (undefined_table) and control
// left via the `else if err != nil` branch before reaching the row-exists
// path. A migration creating `crdt_state` ARMS the defect: the first
// successful read of a persisted CRDT panics.
//
// WHY THESE TESTS CATCH IT WITHOUT A DATABASE: go-sqlmock supplies a real
// database/sql driver, so QueryRowContext(...).Scan(...) runs the genuine
// database/sql machinery and returns a genuine ROW. That is precisely the
// state the pending migration will create, and it is the one input that
// steers GetCRDT into the row-exists branch. No live database, no SQL, no
// network — the row-exists path is reached deterministically.
//
// POLARITY SWITCH (§11.4.115): RED_MODE=1 asserts the defect is PRESENT (the
// row-exists tests then require a panic); RED_MODE unset / 0 is the standing
// GREEN regression guard asserting the defect is ABSENT.
//
// HONEST BOUNDARY on that switch (§11.4.6) — measured 2026-07-28, do not
// overstate it: this FILE cannot be compiled against the pre-fix artifact,
// because TestNewCRDTByType_* reference newCRDTByType, which does not exist
// before the fix. Running RED_MODE=1 here on `git show HEAD~:...manager.go`
// therefore yields a BUILD failure, not a reproduction:
//
//	manager_getcrdt_rowexists_guard_test.go:77:17: undefined: newCRDTByType
//	FAIL dev.helix.agent/internal/ensemble/synchronization [build failed]
//
// The defect WAS reproduced on the pre-fix artifact, by isolating just the
// row-exists drive into a standalone harness (no newCRDTByType reference).
// That run panicked exactly as described above:
//
//	panic: runtime error: invalid memory address or nil pointer dereference
//	[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x5f61bf]
//	...(*SyncManager).GetCRDT(...) manager.go:281
//
// So RED_MODE=1 is the polarity switch for a pre-fix artifact that still
// provides newCRDTByType (e.g. a mutation that empties the factory), NOT for
// the original two-branch code. Reproducing against THAT code requires the
// isolated harness. The GREEN (RED_MODE unset) path is unaffected and is the
// standing guard.

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errUndefinedTable stands in for the PostgreSQL 42P01 undefined_table
// condition that masked this defect in production (the `crdt_state` table
// does not yet exist, so every query failed before reaching the row-exists
// branch).
var errUndefinedTable = errors.New(`pq: relation "crdt_state" does not exist`)

// redModeArmed reports whether the §11.4.115 polarity switch is set to
// reproduce-and-assert-defect-present.
func redModeArmed() bool { return os.Getenv("RED_MODE") == "1" }

func quietLogger() *log.Logger { return log.New(io.Discard, "", 0) }

// TestNewCRDTByType_ConstructsEveryRegisteredType pins the extracted factory
// directly. It needs no database at all: it is the shared construction point
// both GetCRDT branches now depend on, so proving it never returns a
// (nil, nil) pair proves neither branch can obtain a nil CRDT.
func TestNewCRDTByType_ConstructsEveryRegisteredType(t *testing.T) {
	tests := []struct {
		crdtType string
		wantType string
	}{
		{"g_counter", "g_counter"},
		{"pn_counter", "pn_counter"},
		{"g_set", "g_set"},
		{"lww_register", "lww_register"},
	}

	for _, tt := range tests {
		t.Run(tt.crdtType, func(t *testing.T) {
			crdt, err := newCRDTByType(tt.crdtType, "some-key")
			require.NoError(t, err)
			require.NotNil(t, crdt, "factory must never return a nil CRDT with a nil error")
			assert.Equal(t, tt.wantType, crdt.Type())
			assert.Equal(t, "some-key", crdt.Key())

			// The value must be usable as a CRDT immediately — this is the
			// exact call the row-exists branch makes.
			assert.NotPanics(t, func() { _ = crdt.FromJSON([]byte(`{}`)) })
		})
	}
}

// TestNewCRDTByType_UnknownTypeReturnsError pins requirement 3: an
// unrecognised crdt_type is an explicit error, never a nil CRDT handed back
// for the caller to dereference.
func TestNewCRDTByType_UnknownTypeReturnsError(t *testing.T) {
	crdt, err := newCRDTByType("definitely_not_a_crdt", "k")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown CRDT type")
	assert.Nil(t, crdt)
}

// TestGetCRDT_RowExists_HydratesConcreteCRDT is the core guard: it drives
// GetCRDT down the row-exists branch — the branch that panicked pre-fix.
func TestGetCRDT_RowExists_HydratesConcreteCRDT(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Build the persisted state through the REAL type so the fixture cannot
	// drift from the production JSON shape.
	seed := NewGCounter("orders")
	seed.Increment("node-a", 7)
	state, err := seed.ToJSON()
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT state, vector_clock FROM crdt_state`).
		WithArgs("g_counter", "orders").
		WillReturnRows(
			sqlmock.NewRows([]string{"state", "vector_clock"}).
				AddRow(state, []byte(`{"node-a":1}`)),
		)

	sm := NewSyncManager(db, quietLogger(), "node-a")
	defer sm.Close()

	ctx := context.Background()

	if redModeArmed() {
		// Pre-fix artifact: GetCRDT calls FromJSON on the nil CRDT interface.
		require.Panics(t, func() { _, _ = sm.GetCRDT(ctx, "g_counter", "orders") },
			"RED_MODE=1 expects the pre-fix nil-interface panic on the row-exists path")
		return
	}

	got, err := sm.GetCRDT(ctx, "g_counter", "orders")
	require.NoError(t, err)
	require.NotNil(t, got)

	// The returned value is the correct concrete type for the stored
	// crdt_type discriminator...
	counter, ok := got.(*GCounter)
	require.True(t, ok, "row-exists path must construct by crdt_type, got %T", got)

	// ...and it was genuinely hydrated from the persisted state, not merely
	// constructed empty (a construct-but-never-hydrate fix would pass a
	// not-nil check while silently losing the stored value).
	assert.Equal(t, "orders", counter.Key())
	assert.Equal(t, int64(7), counter.Values["node-a"])
	assert.Equal(t, int64(7), counter.Value())

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetCRDT_RowExists_UnknownTypeReturnsErrorNotPanic covers requirement 3
// on the dangerous path: a row persisted under a crdt_type this build does
// not recognise (an older/newer writer, or a corrupted discriminator) must
// surface an error rather than panic the process.
func TestGetCRDT_RowExists_UnknownTypeReturnsErrorNotPanic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT state, vector_clock FROM crdt_state`).
		WithArgs("or_set", "carts").
		WillReturnRows(
			sqlmock.NewRows([]string{"state", "vector_clock"}).
				AddRow([]byte(`{"key":"carts"}`), []byte(`{}`)),
		)

	sm := NewSyncManager(db, quietLogger(), "node-a")
	defer sm.Close()

	ctx := context.Background()

	if redModeArmed() {
		require.Panics(t, func() { _, _ = sm.GetCRDT(ctx, "or_set", "carts") },
			"RED_MODE=1 expects the pre-fix nil-interface panic for an unknown type on the row-exists path")
		return
	}

	var got CRDT
	require.NotPanics(t, func() { got, err = sm.GetCRDT(ctx, "or_set", "carts") })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown CRDT type")
	assert.Nil(t, got)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetCRDT_NoRows_StillCreatesFresh pins the pre-existing row-absent
// behaviour so the restructure cannot regress it.
func TestGetCRDT_NoRows_StillCreatesFresh(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT state, vector_clock FROM crdt_state`).
		WithArgs("g_set", "tags").
		WillReturnRows(sqlmock.NewRows([]string{"state", "vector_clock"}))

	sm := NewSyncManager(db, quietLogger(), "node-a")
	defer sm.Close()

	got, err := sm.GetCRDT(context.Background(), "g_set", "tags")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "g_set", got.Type())
	assert.Equal(t, "tags", got.Key())

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetCRDT_QueryError_IsReturned pins that a genuine query failure (the
// 42P01 undefined_table condition that masked this defect in production)
// still surfaces as an error and is not swallowed by the restructure.
func TestGetCRDT_QueryError_IsReturned(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT state, vector_clock FROM crdt_state`).
		WithArgs("g_counter", "orders").
		WillReturnError(errUndefinedTable)

	sm := NewSyncManager(db, quietLogger(), "node-a")
	defer sm.Close()

	got, err := sm.GetCRDT(context.Background(), "g_counter", "orders")
	require.Error(t, err)
	assert.ErrorIs(t, err, errUndefinedTable)
	assert.Nil(t, got)

	assert.NoError(t, mock.ExpectationsWereMet())
}
