package terraform_inventory

import "encoding/json"

type HostVar struct {
	Hostname string `json:"hostname"`
}

type Group struct {
	Hosts    []string               `json:"hosts"`
	Children []string               `json:"children"`
	Vars     map[string]interface{} `json:"vars"`
}

type Meta struct {
	Hostvars map[string]HostVar `json:"hostvars"`
}

type Inventory struct {
	Meta   *Meta `json:"_meta"`
	Groups map[string]*Group
}

func (i *Inventory) MarshalJSON() ([]byte, error) {

	m := make(map[string]interface{})

	m["_meta"] = i.Meta

	for name, group := range i.Groups {
		m[name] = group
	}

	return json.MarshalIndent(m, "", " ")
}
