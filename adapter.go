package casbinbunadapter

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

var (
	// check if the bunAdapter implements the Adapter interface
	_ persist.Adapter = (*bunAdapter)(nil)
	// check if the bunAdapter implements the BatchAdapter interface
	_ persist.BatchAdapter = (*bunAdapter)(nil)
	// check if the bunAdapter implements the UpdatableAdapter interface
	_ persist.UpdatableAdapter = (*bunAdapter)(nil)
)

type bunAdapter struct {
	db *bun.DB
}

func NewAdapter(driverName, dataSourceName string) (persist.Adapter, error) {
	sqlDB, err := openSqlDB(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	db, err := openBunDB(sqlDB, driverName)
	if err != nil {
		return nil, err
	}

	b, err := newAdapter(db)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func NewAdapterWithSqlDB(sqlDB *sql.DB, driverName string) (persist.Adapter, error) {
	db, err := openBunDB(sqlDB, driverName)
	if err != nil {
		return nil, err
	}

	b, err := newAdapter(db)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func NewAdapterWithBunDB(db *bun.DB) (persist.Adapter, error) {
	b, err := newAdapter(db)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func newAdapter(db *bun.DB) (persist.Adapter, error) {
	b := &bunAdapter{
		db: db,
	}

	if err := b.createTable(); err != nil {
		return nil, err
	}

	runtime.SetFinalizer(b, func(a *bunAdapter) {
		if err := a.db.Close(); err != nil {
			panic(err)
		}
	})

	return b, nil
}

func openSqlDB(driverName, dataSourceName string) (*sql.DB, error) {
	switch driverName {
	case "mysql":
		return sql.Open(driverName, dataSourceName)
	// case "postgres":
	// 	return sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dataSourceName))), nil
	case "mssql":
		return sql.Open(driverName, dataSourceName)
	// case "sqlite3":
	// 	return sql.Open(sqliteshim.ShimName, dataSourceName)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driverName)
	}
}

func openBunDB(sqlDB *sql.DB, driverName string) (*bun.DB, error) {
	switch driverName {
	case "mysql":
		return bun.NewDB(sqlDB, mysqldialect.New()), nil
	case "postgres":
		return bun.NewDB(sqlDB, pgdialect.New()), nil
	case "mssql":
		return bun.NewDB(sqlDB, mssqldialect.New()), nil
	case "sqlite3":
		return bun.NewDB(sqlDB, sqlitedialect.New()), nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driverName)
	}
}

func (a *bunAdapter) createTable() error {
	if _, err := a.db.NewCreateTable().
		Model((*CasbinPolicy)(nil)).
		IfNotExists().
		Exec(context.Background()); err != nil {
		return err
	}
	return nil
}

// LoadPolicy loads all policy rules from the storage.
func (a *bunAdapter) LoadPolicy(model model.Model) error {
	var policies []CasbinPolicy
	err := a.db.NewSelect().
		Model(&policies).
		Scan(context.Background())
	if err != nil {
		return err
	}

	for _, policy := range policies {
		if err := loadPolicyRecord(policy, model); err != nil {
			return err
		}
	}

	return nil
}

func loadPolicyRecord(policy CasbinPolicy, model model.Model) error {
	pType := policy.PType
	sec := pType[:1]
	ok, err := model.HasPolicyEx(sec, pType, policy.filterValues())
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	model.AddPolicy(sec, pType, policy.filterValues())
	return nil
}

// SavePolicy saves all policy rules to the storage.
func (a *bunAdapter) SavePolicy(model model.Model) error {
	policies := make([]CasbinPolicy, 0)

	// go through policy definitions
	for ptype, ast := range model["p"] {
		for _, rule := range ast.Policy {
			policies = append(policies, newCasbinPolicy(ptype, rule))
		}
	}

	// go through role definitions
	for ptype, ast := range model["g"] {
		for _, rule := range ast.Policy {
			policies = append(policies, newCasbinPolicy(ptype, rule))
		}
	}

	return a.savePolicyRecords(policies)
}

func (a *bunAdapter) savePolicyRecords(policies []CasbinPolicy) error {
	// delete existing policies
	if err := a.refreshTable(); err != nil {
		return err
	}

	// bulk insert new policies
	if _, err := a.db.NewInsert().
		Model(&policies).
		Exec(context.Background()); err != nil {
		return err
	}

	return nil
}

// truncate tables
func (a *bunAdapter) refreshTable() error {
	if _, err := a.db.NewTruncateTable().
		Model((*CasbinPolicy)(nil)).
		Exec(context.Background()); err != nil {
		return err
	}
	return nil
}

// AddPolicy adds a policy rule to the storage.
// This is part of the Auto-Save feature.
func (a *bunAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	newPolicy := newCasbinPolicy(ptype, rule)
	if _, err := a.db.NewInsert().
		Model(&newPolicy).
		Exec(context.Background()); err != nil {
		return err
	}
	return nil
}

// AddPolicies adds policy rules to the storage.
// This is part of the Auto-Save feature.
func (a *bunAdapter) AddPolicies(sec string, ptype string, rules [][]string) error {
	policies := make([]CasbinPolicy, 0)
	for _, rule := range rules {
		policies = append(policies, newCasbinPolicy(ptype, rule))
	}
	if _, err := a.db.NewInsert().
		Model(&policies).
		Exec(context.Background()); err != nil {
		return err
	}
	return nil
}

// RemovePolicy removes a policy rule from the storage.
// This is part of the Auto-Save feature.
func (a *bunAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	exisingPolicy := newCasbinPolicy(ptype, rule)
	if err := a.deleteRecord(exisingPolicy); err != nil {
		return err
	}
	return nil
}

// RemovePolicies removes policy rules from the storage.
// This is part of the Auto-Save feature.
func (a *bunAdapter) RemovePolicies(sec string, ptype string, rules [][]string) error {
	return a.db.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, rule := range rules {
			exisingPolicy := newCasbinPolicy(ptype, rule)
			if err := a.deleteRecordInTx(tx, exisingPolicy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *bunAdapter) deleteRecord(existingPolicy CasbinPolicy) error {
	query := a.db.NewDelete().
		Model((*CasbinPolicy)(nil)).
		Where("ptype = ?", existingPolicy.PType)

	values := existingPolicy.filterValuesWithKey()

	return a.delete(query, values)
}

func (a *bunAdapter) deleteRecordInTx(tx bun.Tx, existingPolicy CasbinPolicy) error {
	query := tx.NewDelete().
		Model((*CasbinPolicy)(nil)).
		Where("ptype = ?", existingPolicy.PType)

	values := existingPolicy.filterValuesWithKey()

	return a.delete(query, values)
}

func (a *bunAdapter) delete(query *bun.DeleteQuery, values map[string]string) error {
	for key, value := range values {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}

	if _, err := query.Exec(context.Background()); err != nil {
		return err
	}

	return nil
}

// RemoveFilteredPolicy removes policy rules that match the filter from the storage.
// This is part of the Auto-Save feature.
// This API is explained in the link below:
// https://casbin.org/docs/management-api/#removefilteredpolicy
func (a *bunAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	if err := a.deleteFilteredPolicy(ptype, fieldIndex, fieldValues...); err != nil {
		return err
	}
	return nil
}

func (a *bunAdapter) deleteFilteredPolicy(ptype string, fieldIndex int, fieldValues ...string) error {
	query := a.db.NewDelete().
		Model((*CasbinPolicy)(nil)).
		Where("ptype = ?", ptype)

	// Note that empty string in fieldValues could be any word.
	if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) {
		value := fieldValues[0-fieldIndex]
		if value == "" {
			query = query.Where("v0 LIKE '%'")
		} else {
			query = query.Where("v0 = ?", value)
		}
	}
	if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) {
		value := fieldValues[1-fieldIndex]
		if value == "" {
			query = query.Where("v1 LIKE '%'")
		} else {
			query = query.Where("v1 = ?", value)
		}
	}
	if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) {
		value := fieldValues[2-fieldIndex]
		if value == "" {
			query = query.Where("v2 LIKE '%'")
		} else {
			query = query.Where("v2 = ?", value)
		}
	}
	if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) {
		value := fieldValues[3-fieldIndex]
		if value == "" {
			query = query.Where("v3 LIKE '%'")
		} else {
			query = query.Where("v3 = ?", value)
		}
	}
	if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) {
		value := fieldValues[4-fieldIndex]
		if value == "" {
			query = query.Where("v4 LIKE '%'")
		} else {
			query = query.Where("v4 = ?", value)
		}
	}
	if fieldIndex <= 5 && 5 < fieldIndex+len(fieldValues) {
		value := fieldValues[5-fieldIndex]
		if value == "" {
			query = query.Where("v5 LIKE '%'")
		} else {
			query = query.Where("v5 = ?", value)
		}
	}

	if _, err := query.Exec(context.Background()); err != nil {
		return err
	}

	return nil
}

// UpdatePolicy updates a policy rule from storage.
// This is part of the Auto-Save feature.
func (a *bunAdapter) UpdatePolicy(sec string, ptype string, oldRule, newRule []string) error {
	oldPolicy := newCasbinPolicy(ptype, oldRule)
	newPolicy := newCasbinPolicy(ptype, newRule)
	return a.updateRecord(oldPolicy, newPolicy)
}

func (a *bunAdapter) updateRecord(oldPolicy, newPolicy CasbinPolicy) error {
	query := a.db.NewUpdate().
		Model(&newPolicy).
		Where("ptype = ?", oldPolicy.PType)

	values := oldPolicy.filterValuesWithKey()

	return a.update(query, values)
}

func (a *bunAdapter) updateRecordInTx(tx bun.Tx, oldPolicy, newPolicy CasbinPolicy) error {
	query := tx.NewUpdate().
		Model(&newPolicy).
		Where("ptype = ?", oldPolicy.PType)

	values := oldPolicy.filterValuesWithKey()

	return a.update(query, values)
}

func (a *bunAdapter) update(query *bun.UpdateQuery, values map[string]string) error {
	for key, value := range values {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}

	if _, err := query.Exec(context.Background()); err != nil {
		return err
	}

	return nil
}

// UpdatePolicies updates some policy rules to storage, like db, redis.
func (a *bunAdapter) UpdatePolicies(sec string, ptype string, oldRules, newRules [][]string) error {
	oldPolicies := make([]CasbinPolicy, 0, len(oldRules))
	newPolicies := make([]CasbinPolicy, 0, len(newRules))
	for _, rule := range oldRules {
		oldPolicies = append(oldPolicies, newCasbinPolicy(ptype, rule))
	}
	for _, rule := range newRules {
		newPolicies = append(newPolicies, newCasbinPolicy(ptype, rule))
	}

	return a.db.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for i := range oldPolicies {
			if err := a.updateRecordInTx(tx, oldPolicies[i], newPolicies[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateFilteredPolicies deletes old rules and adds new rules.
func (a *bunAdapter) UpdateFilteredPolicies(sec string, ptype string, newRules [][]string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	newPolicies := make([]CasbinPolicy, 0, len(newRules))
	for _, rule := range newRules {
		newPolicies = append(newPolicies, newCasbinPolicy(ptype, rule))
	}

	tx, err := a.db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return nil, err
	}

	oldPolicies := make([]CasbinPolicy, 0)
	selectQuery := tx.NewSelect().
		Model(&oldPolicies).
		Where("ptype = ?", ptype)
	deleteQuery := tx.NewDelete().
		Model((*CasbinPolicy)(nil)).
		Where("ptype = ?", ptype)

	// Note that empty string in fieldValues could be any word.
	if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) {
		value := fieldValues[0-fieldIndex]
		if value == "" {
			selectQuery = selectQuery.Where("v0 LIKE '%'")
			deleteQuery = deleteQuery.Where("v0 LIKE '%'")
		} else {
			selectQuery = selectQuery.Where("v0 = ?", value)
			deleteQuery = deleteQuery.Where("v0 = ?", value)
		}
	}
	if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) {
		value := fieldValues[1-fieldIndex]
		if value == "" {
			selectQuery = selectQuery.Where("v1 LIKE '%'")
			deleteQuery = deleteQuery.Where("v1 LIKE '%'")
		} else {
			selectQuery = selectQuery.Where("v1 = ?", value)
			deleteQuery = deleteQuery.Where("v1 = ?", value)
		}
	}
	if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) {
		value := fieldValues[2-fieldIndex]
		if value == "" {
			selectQuery = selectQuery.Where("v2 LIKE '%'")
			deleteQuery = deleteQuery.Where("v2 LIKE '%'")
		} else {
			selectQuery = selectQuery.Where("v2 = ?", value)
			deleteQuery = deleteQuery.Where("v2 = ?", value)
		}
	}
	if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) {
		value := fieldValues[3-fieldIndex]
		if value == "" {
			selectQuery = selectQuery.Where("v3 LIKE '%'")
			deleteQuery = deleteQuery.Where("v3 LIKE '%'")
		} else {
			selectQuery = selectQuery.Where("v3 = ?", value)
			deleteQuery = deleteQuery.Where("v3 = ?", value)
		}
	}
	if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) {
		value := fieldValues[4-fieldIndex]
		if value == "" {
			selectQuery = selectQuery.Where("v4 LIKE '%'")
			deleteQuery = deleteQuery.Where("v4 LIKE '%'")
		} else {
			selectQuery = selectQuery.Where("v4 = ?", value)
			deleteQuery = deleteQuery.Where("v4 = ?", value)
		}
	}
	if fieldIndex <= 5 && 5 < fieldIndex+len(fieldValues) {
		value := fieldValues[5-fieldIndex]
		if value == "" {
			selectQuery = selectQuery.Where("v5 LIKE '%'")
			deleteQuery = deleteQuery.Where("v5 LIKE '%'")
		} else {
			selectQuery = selectQuery.Where("v5 = ?", value)
			deleteQuery = deleteQuery.Where("v5 = ?", value)
		}
	}

	// store old policies
	if err := selectQuery.Scan(context.Background()); err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	// delete old policies
	if _, err := deleteQuery.Exec(context.Background()); err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	// create new policies
	if _, err := tx.NewInsert().
		Model(&newPolicies).
		Exec(context.Background()); err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	out := make([][]string, 0, len(oldPolicies))
	for _, policy := range oldPolicies {
		out = append(out, policy.toSlice())
	}

	return out, tx.Commit()
}
