package rethinkadapter

import (
	"fmt"
	"runtime"

	"github.com/casbin/casbin/model"
	"github.com/casbin/casbin/persist"
	r "gopkg.in/rethinkdb/rethinkdb-go.v5"
)

// adapter represents the RethinkDB adapter for policy storage.
type adapter struct {
	session  r.QueryExecutor
	database string
	table    string
}

type policy struct {
	ID    string `gorethink:"id,omitempty"`
	PTYPE string `gorethink:"ptype"`
	V1    string `gorethink:"v1"`
	V2    string `gorethink:"v2"`
	V3    string `gorethink:"v3"`
	V4    string `gorethink:"v4"`
	V5    string `gorethink:"v5"`
}

func finalizer(a *adapter) {
	a.close()
}

// NewAdapter is the constructor for adapter.
func NewAdapter(Sessionvar r.QueryExecutor, database, table string) persist.Adapter {
	a := &adapter{session: Sessionvar, database: database, table: table}
	a.open()
	// Call the destructor when the object is released.
	runtime.SetFinalizer(a, finalizer)
	return a
}

// GetDatabaseName returns the name of the database that the adapter will use
func (a *adapter) GetDatabaseName() string {
	return a.database
}

// GetTableName returns the name of the table that the adapter will use
func (a *adapter) GetTableName() string {
	return a.database
}

// SetDatabaseName sets the database that the adapter will use
func (a *adapter) SetDatabaseName(s string) {
	a.database = s
}

// SetTableName sets the tablet that the adapter will use
func (a *adapter) SetTableName(s string) {
	a.table = s
}

func (a *adapter) close() {
	a.session = nil
}

func (a *adapter) createDatabase() error {
	_, err := r.DBList().Contains(a.database).Do(r.DBCreate(a.database).Exec(a.session)).Run(a.session)
	if err != nil {
		return err
	}
	return nil
}

func (a *adapter) createTable() error {
	_, err := r.DB(a.database).TableList().Contains(a.table).Do(r.DB(a.database).TableCreate(a.table).Exec(a.session)).Run(a.session)
	if err != nil {
		return err
	}
	return nil
}

func (a *adapter) open() {
	if err := a.createDatabase(); err != nil {
		panic(err)
	}

	if err := a.createTable(); err != nil {
		panic(err)
	}
}

//Erase the table data
func (a *adapter) dropTable() error {
	_, err := r.DB(a.database).Table(a.table).Delete().Run(a.session)
	if err != nil {
		panic(err)
	}
	return nil
}

func loadPolicyLine(line policy, model model.Model) {
	if line.PTYPE == "" {
		return
	}

	key := line.PTYPE
	sec := key[:1]

	tokens := []string{}

	if line.V1 != "" {
		tokens = append(tokens, line.V1)
	}

	if line.V2 != "" {
		tokens = append(tokens, line.V2)
	}

	if line.V3 != "" {
		tokens = append(tokens, line.V3)
	}

	if line.V4 != "" {
		tokens = append(tokens, line.V4)
	}

	if line.V5 != "" {
		tokens = append(tokens, line.V5)
	}

	model[sec][key].Policy = append(model[sec][key].Policy, tokens)
}

// LoadPolicy loads policy from database.
func (a *adapter) LoadPolicy(model model.Model) error {
	a.open()

	rows, errn := r.DB(a.database).Table(a.table).Run(a.session)
	if errn != nil {
		fmt.Printf("E: %v\n", errn)
		return errn
	}

	defer rows.Close()
	var output policy

	for rows.Next(&output) {
		loadPolicyLine(output, model)
	}
	return nil
}

func (a *adapter) writeTableLine(ptype string, rule []string) policy {
	items := policy{
		PTYPE: ptype,
	} //map[string]string{"PTYPE": ptype, "V1": "", "V2": "", "V3": "", "V4": ""}
	for i := 0; i < len(rule); i++ {
		switch i {
		case 0:
			items.V1 = rule[i]
		case 1:
			items.V2 = rule[i]
		case 2:
			items.V3 = rule[i]
		case 3:
			items.V4 = rule[i]
		case 4:
			items.V5 = rule[i]
		}
	}
	return items
}

// SavePolicy saves policy to database.
func (a *adapter) SavePolicy(model model.Model) error {
	a.open()
	a.dropTable()
	var lines []policy

	for PTYPE, ast := range model["p"] {
		for _, rule := range ast.Policy {
			line := a.writeTableLine(PTYPE, rule)
			lines = append(lines, line)
		}
	}

	for PTYPE, ast := range model["g"] {
		for _, rule := range ast.Policy {
			line := a.writeTableLine(PTYPE, rule)
			lines = append(lines, line)
		}
	}
	_, err := r.DB(a.database).Table(a.table).Insert(lines).Run(a.session)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	return nil
}

//AddPolicy for adding a new policy to rethinkdb
func (a *adapter) AddPolicy(sec string, PTYPE string, policys []string) error {
	line := a.writeTableLine(PTYPE, policys)
	_, err := r.DB(a.database).Table(a.table).Insert(line).Run(a.session)
	if err != nil {
		return err
	}
	return nil
}

//RemovePolicy for removing a policy rule from rethinkdb
func (a *adapter) RemovePolicy(sec string, PTYPE string, policys []string) error {
	line := a.writeTableLine(PTYPE, policys)
	_, err := r.DB(a.database).Table(a.table).Filter(line).Delete().Run(a.session)
	if err != nil {
		return err
	}
	return nil
}

//RemoveFilteredPolicy for removing filtered policy
func (a *adapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	var selector policy
	selector.PTYPE = ptype

	if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) {
		selector.V1 = fieldValues[0-fieldIndex]
	}
	if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) {
		selector.V2 = fieldValues[1-fieldIndex]
	}
	if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) {
		selector.V3 = fieldValues[2-fieldIndex]
	}
	if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) {
		selector.V4 = fieldValues[3-fieldIndex]
	}
	if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) {
		selector.V5 = fieldValues[4-fieldIndex]
	}

	_, err := r.DB(a.database).Table(a.table).Filter(selector).Delete().Run(a.session)
	if err != nil {
		panic(err)
	}
	return nil
}
