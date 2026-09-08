package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

type Store struct{ root string }

func New(root string) *Store { return &Store{root: root} }
func (s *Store) Init() error {
	for _, d := range []string{"instances", "plans"} {
		if err := os.MkdirAll(filepath.Join(s.root, d), 0750); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) SaveInstance(v model.Instance) error {
	return atomicJSON(filepath.Join(s.root, "instances", v.ID+".json"), v)
}
func (s *Store) GetInstance(id string) (model.Instance, error) {
	var v model.Instance
	if !safeID(id) {
		return v, errors.New("invalid instance id")
	}
	err := readJSON(filepath.Join(s.root, "instances", id+".json"), &v)
	return v, err
}
func (s *Store) ListInstances() ([]model.Instance, error) {
	ents, err := os.ReadDir(filepath.Join(s.root, "instances"))
	if err != nil {
		return nil, err
	}
	out := []model.Instance{}
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var v model.Instance
		if err := readJSON(filepath.Join(s.root, "instances", e.Name()), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Store) SavePlan(v model.Plan) error {
	return atomicJSON(filepath.Join(s.root, "plans", v.ID+".json"), v)
}
func (s *Store) GetPlan(id string) (model.Plan, error) {
	var v model.Plan
	if !safeID(id) {
		return v, errors.New("invalid plan id")
	}
	err := readJSON(filepath.Join(s.root, "plans", id+".json"), &v)
	return v, err
}
func (s *Store) DeletePlan(id string) error {
	if !safeID(id) {
		return errors.New("invalid plan id")
	}
	err := os.Remove(filepath.Join(s.root, "plans", id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func safeID(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func readJSON(p string, v any) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("decode %s: %w", p, err)
	}
	return nil
}
func atomicJSON(p string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0640); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, p); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
