package repository_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/testutil/pgtest"
)

// widget is a test-only model; its table is created by the test itself in
// the test's private database.
type widget struct {
	repository.Model
	Name string
}

const createWidgets = `CREATE TABLE widgets (
	id         uuid PRIMARY KEY,
	name       text NOT NULL,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
)`

func TestModelAssignsUUIDv7OnCreate(t *testing.T) {
	t.Parallel()

	db := pgtest.New(t)
	if err := db.Exec(createWidgets).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	first := widget{Name: "first"}
	second := widget{Name: "second"}
	for _, w := range []*widget{&first, &second} {
		if err := db.Create(w).Error; err != nil {
			t.Fatalf("create %s: %v", w.Name, err)
		}
	}

	if first.ID == uuid.Nil {
		t.Fatal("ID not assigned on create")
	}
	if got := first.ID.Version(); got != 7 {
		t.Errorf("ID version = %d, want 7", got)
	}
	if first.ID.String() >= second.ID.String() {
		t.Errorf("IDs not time-ordered: %s then %s", first.ID, second.ID)
	}

	var stored widget
	if err := db.First(&stored, "id = ?", first.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if stored.Name != "first" {
		t.Errorf("stored name = %q, want %q", stored.Name, "first")
	}
}

func TestModelKeepsExplicitID(t *testing.T) {
	t.Parallel()

	db := pgtest.New(t)
	if err := db.Exec(createWidgets).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	w := widget{Model: repository.Model{ID: id}, Name: "preset"}
	if err := db.Create(&w).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if w.ID != id {
		t.Errorf("ID = %s, want %s", w.ID, id)
	}
}
