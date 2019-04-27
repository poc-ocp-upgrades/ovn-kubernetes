package util

import (
	"fmt"
	"strings"
	"github.com/coreos/go-iptables/iptables"
)

type IPTablesHelper interface {
	ListChains(string) ([]string, error)
	NewChain(string, string) error
	Exists(string, string, ...string) (bool, error)
	Insert(string, string, int, ...string) error
}

func NewWithProtocol(proto iptables.Protocol) (IPTablesHelper, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return iptables.NewWithProtocol(proto)
}

type FakeTable map[string][]string

func newFakeTable() *FakeTable {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return &FakeTable{}
}
func (t *FakeTable) String() string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return fmt.Sprintf("%v", *t)
}
func (t *FakeTable) getChain(chainName string) ([]string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	chain, ok := (*t)[chainName]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist", chainName)
	}
	return chain, nil
}

type FakeIPTables struct {
	proto	iptables.Protocol
	tables	map[string]*FakeTable
}

func NewFakeWithProtocol(proto iptables.Protocol) (*FakeIPTables, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	ipt := &FakeIPTables{proto: proto, tables: make(map[string]*FakeTable)}
	ipt.tables["filter"] = newFakeTable()
	ipt.tables["nat"] = newFakeTable()
	return ipt, nil
}
func (f *FakeIPTables) getTable(tableName string) (*FakeTable, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	table, ok := f.tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}
	return table, nil
}
func (f *FakeIPTables) ListChains(tableName string) ([]string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	table, ok := f.tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table does not exist")
	}
	chains := make([]string, len(*table))
	for c := range *table {
		chains = append(chains, c)
	}
	return chains, nil
}
func (f *FakeIPTables) NewChain(tableName, chainName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	table, err := f.getTable(tableName)
	if err != nil {
		return err
	}
	if _, err := table.getChain(chainName); err == nil {
		return err
	}
	(*table)[chainName] = nil
	return nil
}
func (f *FakeIPTables) Exists(tableName, chainName string, rulespec ...string) (bool, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	table, err := f.getTable(tableName)
	if err != nil {
		return false, err
	}
	chain, err := table.getChain(chainName)
	if err != nil {
		return false, err
	}
	matchRule := strings.Join(rulespec, " ")
	for _, rule := range chain {
		if rule == matchRule {
			return true, nil
		}
	}
	return false, nil
}
func (f *FakeIPTables) Insert(tableName, chainName string, pos int, rulespec ...string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	table, err := f.getTable(tableName)
	if err != nil {
		return err
	}
	if pos < 1 {
		return fmt.Errorf("invalid rule position %d", pos)
	}
	rule := strings.Join(rulespec, " ")
	chain, _ := table.getChain(chainName)
	if pos >= len(chain) {
		(*table)[chainName] = append(chain, rule)
	} else {
		first := append(chain[:pos-1], rule)
		(*table)[chainName] = append(first, chain[pos-1:]...)
	}
	return nil
}
func (f *FakeIPTables) MatchState(tables map[string]FakeTable) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if len(tables) != len(f.tables) {
		return fmt.Errorf("expeted %d tables, got %d", len(tables), len(f.tables))
	}
	for tableName, table := range tables {
		foundTable, err := f.getTable(tableName)
		if err != nil {
			return err
		}
		if len(table) != len(*foundTable) {
			var keys, foundKeys []string
			for k := range table {
				keys = append(keys, k)
			}
			for k := range *foundTable {
				foundKeys = append(foundKeys, k)
			}
			return fmt.Errorf("expected %v chains from table %s, got %v", keys, tableName, foundKeys)
		}
		for chainName, chain := range table {
			foundChain, err := foundTable.getChain(chainName)
			if err != nil {
				return err
			}
			if len(chain) != len(foundChain) {
				return fmt.Errorf("expected %d %v rules in chain %s/%s, got %d %v", len(chain), chain, tableName, chainName, len(foundChain), foundChain)
			}
			for i, rule := range chain {
				if rule != foundChain[i] {
					return fmt.Errorf("expected rule %q at pos %d in chain %s/%s, got %q", rule, i, tableName, chainName, foundChain[i])
				}
			}
		}
	}
	return nil
}
