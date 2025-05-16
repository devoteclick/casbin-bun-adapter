package casbinbunadapter

import (
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/persist"
	"github.com/casbin/casbin/v2/util"
)

func testGetPolicy(t *testing.T, e *casbin.Enforcer, want [][]string) {
	got, err := e.GetPolicy()
	if err != nil {
		t.Fatal(err)
	}

	if !util.Array2DEquals(want, got) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func initPolicy(t *testing.T, adapter persist.Adapter) {
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", "testdata/rbac_policy.csv")
	if err != nil {
		panic(err)
	}

	if err := adapter.SavePolicy(e.GetModel()); err != nil {
		panic(err)
	}

	e.ClearPolicy()
	testGetPolicy(t, e, [][]string{})

	if err := adapter.LoadPolicy(e.GetModel()); err != nil {
		panic(err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}},
	)
}

func initAdapter(t *testing.T, driverName, dataSourceName string) persist.Adapter {
	a, err := NewAdapter(driverName, dataSourceName)
	if err != nil {
		panic(err)
	}

	initPolicy(t, a)

	return a
}

func testSaveLoad(t *testing.T, a persist.Adapter) {
	initPolicy(t, a)

	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}},
	)
}

func testAutoSave(t *testing.T, a persist.Adapter) {
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	e.EnableAutoSave(false)

	if _, err := e.AddPolicy("alice", "data1", "read"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}},
	)

	e.EnableAutoSave(true)

	if _, err := e.AddPolicy("alice", "data1", "write"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}, {"alice", "data1", "write"}},
	)

	if _, err := e.RemovePolicy("alice", "data1", "write"); err != nil {
		t.Fatalf("failed to remove policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}},
	)

	if _, err := e.RemoveFilteredPolicy(0, "data2_admin"); err != nil {
		t.Fatalf("failed to remove filtered policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}},
	)
}

func TestBunAdapters(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	testSaveLoad(t, a)
	testAutoSave(t, a)

	a = initAdapter(t, "postgres", "postgres://postgres:postgres@localhost:5432/test?sslmode=disable")
	testSaveLoad(t, a)
	testAutoSave(t, a)

	a = initAdapter(t, "sqlite3", "file::memory:?cache=shared")
	testSaveLoad(t, a)
	testAutoSave(t, a)

	a = initAdapter(t, "mssql", "sqlserver://sa:Password123@localhost:1433?database=master")
	testSaveLoad(t, a)
	testAutoSave(t, a)
}

func TestBunAdapter_AddPolicy(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	if _, err := e.AddPolicy("jack", "data1", "read"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}, {"jack", "data1", "read"}},
	)
}

func TestBunAdapter_AddPolicies(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	if _, err := e.AddPolicies([][]string{{"jack", "data1", "read"}, {"jill", "data2", "write"}}); err != nil {
		t.Fatalf("failed to add policies: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{
			{"alice", "data1", "read"},
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
			{"jack", "data1", "read"},
			{"jill", "data2", "write"},
		},
	)
}

func TestBunAdapter_RemovePolicy(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	if _, err := e.RemovePolicy("alice", "data1", "read"); err != nil {
		t.Fatalf("failed to remove policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}},
	)
}

func TestBunAdapter_RemovePolicies(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	if _, err := e.RemovePolicies([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}}); err != nil {
		t.Fatalf("failed to remove policies: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{{"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}},
	)
}

func TestBunAdapter_RemoveFilteredPolicy(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	// 1. check if the policy with alice is all removed
	if _, err := e.AddPolicy("alice", "data1", "write"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"alice", "data1", "read"},
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
			{"alice", "data1", "write"},
		},
	)
	if _, err := e.RemoveFilteredPolicy(0, "alice"); err != nil {
		t.Fatalf("failed to remove filtered policy: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
		},
	)
	// 2. check if the policy with data1 is all removed
	if _, err := e.AddPolicies([][]string{{"alice", "data1", "read"}, {"alice", "data1", "write"}, {"alice", "data2", "read"}, {"alice", "data2", "write"}}); err != nil {
		t.Fatalf("failed to add policies: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
			{"alice", "data1", "read"},
			{"alice", "data1", "write"},
			{"alice", "data2", "read"},
			{"alice", "data2", "write"},
		},
	)
	if _, err := e.RemoveFilteredPolicy(1, "data1"); err != nil {
		t.Fatalf("failed to remove filtered policy: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
			{"alice", "data2", "read"},
			{"alice", "data2", "write"},
		},
	)
	// 3. check if the policy with alice and data2 is all removed
	if _, err := e.RemoveFilteredPolicy(0, "alice", "data2"); err != nil {
		t.Fatalf("failed to remove filtered policy: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
		},
	)
	// 4. check if the all policies are removed when fieldValues is empty
	if _, err := e.RemoveFilteredPolicy(0, ""); err != nil {
		t.Fatalf("failed to remove filtered policy: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(t, e, [][]string{})
}

func TestBunAdapter_UpdatePolicy(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	if _, err := e.UpdatePolicy(
		[]string{"alice", "data1", "read"},
		[]string{"alice", "data1", "write"},
	); err != nil {
		t.Fatalf("failed to update policy: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{
			{"alice", "data1", "write"},
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
		},
	)
}

func TestBunAdapter_UpdatePolicies(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	if _, err := e.UpdatePolicies(
		[][]string{{"alice", "data1", "write"}, {"bob", "data2", "write"}},
		[][]string{{"alice", "data1", "read"}, {"bob", "data2", "read"}},
	); err != nil {
		t.Fatalf("failed to update policies: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}
	testGetPolicy(
		t,
		e,
		[][]string{
			{"alice", "data1", "read"},
			{"bob", "data2", "read"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
		},
	)
}

func TestBunAdapter_UpdateFilteredPolicies(t *testing.T) {
	a := initAdapter(t, "mysql", "root:root@tcp(127.0.0.1:3306)/test")
	e, err := casbin.NewEnforcer("testdata/rbac_model.conf", a)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	// 1. check if the policy with alice is all updated
	if _, err := e.AddPolicy("alice", "data1", "write"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"alice", "data1", "read"},
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
			{"alice", "data1", "write"},
		},
	)
	if _, err := e.UpdateFilteredPolicies(
		[][]string{{"alice", "data3", "read"}, {"alice", "data3", "write"}},
		0,
		"alice",
	); err != nil {
		t.Fatalf("failed to update filtered policies: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"bob", "data2", "write"},
			{"data2_admin", "data2", "read"},
			{"data2_admin", "data2", "write"},
			{"alice", "data3", "read"},
			{"alice", "data3", "write"},
		},
	)
	// 2. check if the policy with data2 and write is all updated
	if _, err := e.UpdateFilteredPolicies(
		[][]string{{"bob", "data2", "delete"}, {"data2_admin", "data2", "delete"}},
		1,
		"data2",
		"write",
	); err != nil {
		t.Fatalf("failed to update filtered policies: %v", err)
	}
	_ = e.LoadPolicy()
	testGetPolicy(
		t,
		e,
		[][]string{
			{"data2_admin", "data2", "read"},
			{"alice", "data3", "read"},
			{"alice", "data3", "write"},
			{"bob", "data2", "delete"},
			{"data2_admin", "data2", "delete"},
		},
	)
}
