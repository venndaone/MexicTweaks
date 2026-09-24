package main

// Utilities: registry snapshot / restore / compare helpers.

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// ---------- Registry helpers ----------

type RegVal struct {
	Root   string `json:"root"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
	Type   uint32 `json:"type"`
	Str    string `json:"str,omitempty"`
	Num    uint64 `json:"num,omitempty"`
}

func rootKey(r string) registry.Key {
	if r == "HKLM" {
		return registry.LOCAL_MACHINE
	}
	return registry.CURRENT_USER
}

func regSnapshot(root, path, name string) RegVal {
	v := RegVal{Root: root, Path: path, Name: name}
	k, err := registry.OpenKey(rootKey(root), path, registry.QUERY_VALUE)
	if err != nil {
		return v
	}
	defer k.Close()
	_, typ, err := k.GetValue(name, nil)
	if err != nil {
		return v
	}
	switch typ {
	case registry.SZ, registry.EXPAND_SZ:
		s, _, e := k.GetStringValue(name)
		if e == nil {
			v.Exists, v.Type, v.Str = true, typ, s
		}
	case registry.DWORD, registry.QWORD:
		n, _, e := k.GetIntegerValue(name)
		if e == nil {
			v.Exists, v.Type, v.Num = true, typ, n
		}
	}
	return v
}

func regRestore(v RegVal) error {
	if !v.Exists {
		k, err := registry.OpenKey(rootKey(v.Root), v.Path, registry.SET_VALUE)
		if err != nil {
			return nil
		}
		defer k.Close()
		k.DeleteValue(v.Name)
		return nil
	}
	k, _, err := registry.CreateKey(rootKey(v.Root), v.Path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	switch v.Type {
	case registry.SZ:
		return k.SetStringValue(v.Name, v.Str)
	case registry.EXPAND_SZ:
		return k.SetExpandStringValue(v.Name, v.Str)
	case registry.DWORD:
		return k.SetDWordValue(v.Name, uint32(v.Num))
	case registry.QWORD:
		return k.SetQWordValue(v.Name, v.Num)
	}
	return nil
}

func regSet(root, path, name string, sz bool, s string, d uint32) error {
	k, _, err := registry.CreateKey(rootKey(root), path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if sz {
		return k.SetStringValue(name, s)
	}
	return k.SetDWordValue(name, d)
}

func regMatches(root, path, name string, sz bool, s string, d uint32) bool {
	k, err := registry.OpenKey(rootKey(root), path, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	if sz {
		got, _, err := k.GetStringValue(name)
		return err == nil && strings.EqualFold(got, s)
	}
	got, _, err := k.GetIntegerValue(name)
	return err == nil && uint32(got) == d
}

// regDisplay returns a human readable value for backups and change logs.
func regDisplay(root, path, name string) string {
	v := regSnapshot(root, path, name)
	return v.Display()
}

func (v RegVal) Display() string {
	if !v.Exists {
		return "(not set)"
	}
	switch v.Type {
	case registry.SZ, registry.EXPAND_SZ:
		return `"` + v.Str + `"`
	default:
		return fmt.Sprintf("%d (0x%X)", v.Num, v.Num)
	}
}

func regLabel(root, path, name string) string { return root + `\` + path + ` → ` + name }
