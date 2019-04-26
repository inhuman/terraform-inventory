package terraform_inventory

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/terraform"
	"github.com/pkg/errors"
	"reflect"
	"strings"
)

func Convert(project string, state *terraform.State) (*YmlInventory, error) {

	i := &YmlInventory{
		"all": &Group{
			Children: make(map[string]*Group),
			Vars:     make(map[string]interface{}),
			Hosts:    make(map[string]map[string]string),
		},
	}

	if len(state.Modules) < 1 {
		return nil, errors.New("modules is empty")
	}

	for _, m := range state.Modules {
		if len(m.Path) < 1 {
			return nil, errors.New(fmt.Sprintf("paths is empty module %+v\n", m))
		}

		if (len(m.Path) < 2) && len(state.Modules) > 1 {
			fmt.Printf("module continue with path: %s\n", m.Path)
			continue
		}

		outputs := getOutputs(m, state.Modules)

		if outputs["meta"] == nil {
			return nil, errors.New(fmt.Sprintf("outputs has no meta in module %+v\n", m))
		}

		if outputs["meta"].Value == nil {
			return nil, errors.New(fmt.Sprintf("outputs meta has no value in module %+v\n", m))
		}

		groupValues, ok := outputs["meta"].Value.(map[string]interface{})
		if !ok {
			fmt.Printf("outputs meta value not map in module %+v\n", m)
			return nil, errors.New(fmt.Sprintf("outputs meta value not map in module %+v\n", m))
		}

		if groupValues["group"] == "" {
			return nil, errors.New(fmt.Sprintf("outputs meta value has no group in module %+v\n", m))
		}

		if m.Resources == nil {
			return nil, errors.New(fmt.Sprintf("resources is empty in module %+v\n", m))
		}

		groupName := groupValues["group"].(string)
		groupValuesWithoutGroupName := groupValues

		for resourceName, resource := range m.Resources {
			if !isVm(resourceName) {
				continue
			}

			if (*i)["all"].Children[groupName] == nil {

				delete(groupValuesWithoutGroupName, "group")

				(*i)["all"].Children[groupName] = &Group{
					Hosts:    map[string]map[string]string{},
					Vars:     groupValuesWithoutGroupName,
					Children: make(map[string]*Group),
				}

			}

			if resource.Primary == nil {
				return nil, errors.New(fmt.Sprintf("resource %s (%s) has not %s\n", resourceName, groupName, "primary"))
			}

			if resource.Primary.Attributes == nil {
				return nil, errors.New(fmt.Sprintf("resource %s (%s) has not %s\n", resourceName, groupName, "attributes"))
			}

			attrs := resource.Primary.Attributes

			if (attrs["guest_ip_addresses.0"] == "") && (attrs["default_ip_address"] == "") {
				return nil, errors.New(fmt.Sprintf("resource %s (%s) has not %s and %s attributes\n",
					resourceName, groupName, "guest_ip_addresses.0", "default_ip_address"))
			}

			if attrs["name"] == "" {
				return nil, errors.New(fmt.Sprintf("resource %s (%s) has not %s\n", resourceName, groupName, "name"))
			}

			var host string
			if attrs["guest_ip_addresses.0"] != "" {
				host = attrs["guest_ip_addresses.0"]
			} else {
				host = attrs["default_ip_address"]
			}
			(*i)["all"].Children[groupName].Hosts[host] = map[string]string{"hostname": attrs["name"]}
			(*i)[groupName] = &Group{
				Hosts: map[string]map[string]string{
					host: {"hostname": attrs["name"]},
				},
				Vars: groupValuesWithoutGroupName,
			}
		}
	}

	return i, nil
}

func Run(project string, state []byte) (*YmlInventory, error) {

	tfState := &terraform.State{}

	if err := json.Unmarshal(state, tfState); err != nil {
		return nil, err
	}

	i, err := Convert(project, tfState)
	if err != nil {
		return nil, err
	}

	return i, nil
}

func getVmPrefixes() []string {
	return []string{
		"vsphere_virtual_machine.host",
	}
}

func isVm(name string) bool {

	for _, prefix := range getVmPrefixes() {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func getOutputs(module *terraform.ModuleState, modules []*terraform.ModuleState) map[string]*terraform.OutputState {

	if module == nil {
		return nil
	}

	if len(module.Outputs) > 0 {
		return module.Outputs
	}

	path := module.Path
	parentPath := path[:len(path)-1]
	fmt.Printf("parent path: %+v\n", parentPath)

	var parent *terraform.ModuleState

	for _, m := range modules {
		if reflect.DeepEqual(m.Path, parentPath) {
			parent = m
		}
	}

	return getOutputs(parent, modules)
}
