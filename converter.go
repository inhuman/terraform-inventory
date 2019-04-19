package terraform_inventory

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/terraform/terraform"
	"github.com/pkg/errors"
	"reflect"
	"strings"
)

func Convert(project string, state *terraform.State) (*Inventory, error) {

	i := &Inventory{
		Groups: map[string]*Group{
			"all": {
				Hosts:    []string{},
				Children: []string{},
				Vars: map[string]interface{}{
					"project": project,
				},
			},
		},
	}

	if len(state.Modules) < 1 {
		return nil, errors.New("modules is empty")
	}

	for _, m := range state.Modules {
		if len(m.Path) < 1 {
			return nil, errors.New(fmt.Sprintf("paths is empty module %+v\n", m))
		}

		// if path contains only / - skip
		if len(m.Path) < 2 {
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

			fmt.Println("scanning resource:", resourceName)

			if i.Groups[groupName] == nil {

				delete(groupValuesWithoutGroupName, "group")

				i.Groups[groupName] = &Group{
					Hosts:    []string{},
					Vars:     groupValuesWithoutGroupName,
					Children: []string{},
				}

				if i.Groups["all"] == nil {
					i.Groups["all"] = &Group{
						Children: []string{
							groupName,
						},
						Vars:  make(map[string]interface{}),
						Hosts: []string{},
					}
				} else {
					fmt.Println("group name", groupName)
					i.Groups["all"].Children = append(i.Groups["all"].Children, groupName)
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
			i.Groups[groupName].Hosts = append(i.Groups[groupName].Hosts, host)

			if i.Meta == nil {
				i.Meta = &Meta{}
			}

			if i.Meta.Hostvars == nil {
				i.Meta.Hostvars = make(map[string]HostVar)
			}
			i.Meta.Hostvars[host] = HostVar{
				Hostname: attrs["name"],
			}
		}
	}

	return i, nil
}

func Run(consulAddr, clusterTfStatePrefix, project string) (*Inventory, error) {

	stateBytes, err := getState(consulAddr, clusterTfStatePrefix, project)
	if err != nil {
		return nil, err
	}

	tfState := &terraform.State{}

	if err := json.Unmarshal(stateBytes, tfState); err != nil {
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

	if len(module.Path) == 1 {
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

func getState(consulAddr, clusterTfStatePrefix, project string) ([]byte, error) {

	client, err := api.NewClient(&api.Config{
		Address: consulAddr,
		Scheme:  "http",
	})

	if err != nil {
		return nil, err
	}

	k, _, err := client.KV().Get(strings.Join([]string{clusterTfStatePrefix, project}, ":"), &api.QueryOptions{
		Datacenter: "infra1",
	})
	if err != nil {
		return nil, err
	}

	if k != nil {
		return k.Value, nil
	}

	return nil, nil
}
