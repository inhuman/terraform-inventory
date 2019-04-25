package terraform_inventory

type HostVar struct {
	Hostname string `json:"hostname"`
}

type Group struct {
	Hosts    map[string]map[string]string `yaml:"hosts,omitempty"`
	Children map[string]*Group            `yaml:"children,omitempty"`
	Vars     map[string]interface{}       `yaml:"vars,omitempty"`
}

type YmlInventory map[string]*Group
