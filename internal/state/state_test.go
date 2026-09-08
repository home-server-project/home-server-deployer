package state

import (
	"github.com/home-server-project/home-server-deployer/internal/model"
	"testing"
	"time"
)

func TestInstanceRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	in := model.Instance{ID: "alpha-smoke", CatalogID: "alpha-smoke", InstalledAt: time.Now(), Resources: map[string]model.ResourceState{}}
	if err := s.SaveInstance(in); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetInstance("alpha-smoke")
	if err != nil {
		t.Fatal(err)
	}
	if got.CatalogID != in.CatalogID {
		t.Fatal(got)
	}
}
