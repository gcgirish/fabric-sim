// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package topo

import (
	"fmt"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
)

const (
	generatedHeader = "# Generated by fabric-sim-topo utility! Do NOT edit!\n\n"
	agentPortOffset = 20000
)

// Recipe is a container for holding one of the supported simulated topology recipes
type Recipe struct {
	SuperSpineFabric *SuperSpineFabric `mapstructure:"superspine_fabric" yaml:"superspine_fabric"`
	AccessFabric     *AccessFabric     `mapstructure:"access_fabric" yaml:"access_fabric"`
	PlainFabric      *PlainFabric      `mapstructure:"plain_fabric" yaml:"plain_fabric"`
	// Add more recipes here
}

// SuperSpineFabric is a recipe for creating simulated 4 rack fabric with superspines
type SuperSpineFabric struct {
	// Add any parametrization here, if needed
}

// AccessFabric is a recipe for creating simulated access fabric with spines and paired leaves
type AccessFabric struct {
	Spines         int `mapstructure:"spines" yaml:"spines"`
	SpinePortCount int `mapstructure:"spine_port_count" yaml:"spine_port_count"`
	LeafPairs      int `mapstructure:"leaf_pairs" yaml:"leaf_pairs"`
	LeafPortCount  int `mapstructure:"leaf_port_count" yaml:"leaf_port_count"`
	SpineTrunk     int `mapstructure:"spine_trunk" yaml:"spine_trunk"`
	PairTrunk      int `mapstructure:"pair_trunk" yaml:"pair_trunk"`
	HostsPerPair   int `mapstructure:"hosts_per_pair" yaml:"hosts_per_pair"`
}

// PlainFabric is a recipe for creating simulated plain leaf-spine fabric
type PlainFabric struct {
	Spines         int `mapstructure:"spines" yaml:"spines"`
	SpinePortCount int `mapstructure:"spine_port_count" yaml:"spine_port_count"`
	Leaves         int `mapstructure:"leaves" yaml:"leaves"`
	LeafPortCount  int `mapstructure:"leaf_port_count" yaml:"leaf_port_count"`
	SpineTrunk     int `mapstructure:"spine_trunk" yaml:"spine_trunk"`
	HostsPerLeaf   int `mapstructure:"hosts_per_leaf" yaml:"hosts_per_leaf"`
}

// GenerateTopology loads the specified topology recipe YAML file and uses the recipe to
// generate a fully elaborated topology YAML file that can be loaded via LoadTopology
func GenerateTopology(recipePath string, topologyPath string) error {
	log.Infof("Loading topology recipe from %s", recipePath)
	recipe := &Recipe{}
	if err := loadRecipeFile(recipePath, recipe); err != nil {
		return err
	}

	var topology *Topology
	switch {
	case recipe.SuperSpineFabric != nil:
		topology = GenerateSuperSpineFabric(recipe.SuperSpineFabric)
	case recipe.AccessFabric != nil:
		topology = GenerateAccessFabric(recipe.AccessFabric)
	case recipe.PlainFabric != nil:
		topology = GeneratePlainFabric(recipe.PlainFabric)
	default:
		return errors.NewInvalid("no supported topology recipe found")
	}
	return saveTopologyFile(topology, topologyPath)
}

// Loads the specified topology recipe YAML file
func loadRecipeFile(path string, recipe *Recipe) error {
	cfg, err := readConfig(path)
	if err != nil {
		return err
	}
	return cfg.Unmarshal(recipe)
}

// Saves the given topology as YAML in the specified file path; stdout if -
func saveTopologyFile(topology *Topology, path string) error {
	cfg := viper.New()
	cfg.Set("Devices", topology.Devices)
	cfg.Set("links", topology.Links)
	cfg.Set("hosts", topology.Hosts)

	// Create a temporary file and schedule it for removal on exit
	file, err := os.CreateTemp("", "topology*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()

	// Write the configuration to the temporary file
	if err = cfg.WriteConfigAs(file.Name()); err != nil {
		return err
	}

	// Now copy the file to the intended destination; stdout if -
	buffer, err := ioutil.ReadFile(file.Name())
	if err != nil {
		return err
	}

	output := os.Stdout
	if path != "-" {
		output, err = os.Create(path)
		if err != nil {
			return err
		}
		defer output.Close()
	}

	// Write the header comment to the path first
	if _, err = fmt.Fprint(output, generatedHeader); err != nil {
		return err
	}

	// Then append the copy of the YAML content
	if _, err = fmt.Fprint(output, string(buffer)); err != nil {
		return err
	}
	return nil
}

// Utility entities and functions to be shared among the various builders

// Builder hods state to assist generating various fabric topologies
type Builder struct {
	agentPort int32
	nextPort  map[string]int
	minPort   map[string]int
	maxPort   map[string]int
}

// NewBuilder creates a new topology builder context
func NewBuilder() *Builder {
	return &Builder{
		nextPort: make(map[string]int),
		minPort:  make(map[string]int),
		maxPort:  make(map[string]int),
	}
}

// NextAgentPort reserves the next available agent port and returns it
func (b *Builder) NextAgentPort() int32 {
	agentPort := b.agentPort + agentPortOffset
	b.agentPort++
	return agentPort
}

// NextDevicePortID reserves the next available port ID and returns it
func (b *Builder) NextDevicePortID(deviceID string) string {
	portNumber, ok := b.nextPort[deviceID]
	if !ok {
		portNumber = 1
	}
	portID := fmt.Sprintf("%s/%d", deviceID, portNumber)
	b.nextPort[deviceID] = portNumber + 1

	// Wrap around to the min port range
	if b.nextPort[deviceID] > b.maxPort[deviceID] {
		b.nextPort[deviceID] = b.minPort[deviceID]
	}
	return portID
}

// Create a switch with the specified number of ports
func createSwitch(deviceID string, portCount int, builder *Builder, topology *Topology, pos *GridPosition) Device {
	device := Device{
		ID:        deviceID,
		Type:      "switch",
		AgentPort: builder.NextAgentPort(),
		Stopped:   false,
		Ports:     createPorts(portCount),
		Pos:       pos,
	}
	builder.minPort[deviceID] = 1
	builder.maxPort[deviceID] = portCount
	topology.Devices = append(topology.Devices, device)
	return device
}

// Create a list of ports
func createPorts(portCount int) []Port {
	ports := make([]Port, 0, portCount)
	for i := uint32(1); i <= uint32(portCount); i++ {
		port := Port{
			Number:    i,
			SDNNumber: i + 200,
			Speed:     "100GB",
		}
		ports = append(ports, port)
	}
	return ports
}

// Create a trunk of specified number of links between two Devices
func createLinkTrunk(src string, tgt string, count int, builder *Builder, topology *Topology) {
	for i := 0; i < count; i++ {
		link := Link{
			SrcPortID:      builder.NextDevicePortID(src),
			TgtPortID:      builder.NextDevicePortID(tgt),
			Unidirectional: false,
		}
		topology.Links = append(topology.Links, link)
	}
}

// Create the specified number of hosts, each with two NICs connected to the two switches
func createRackHosts(rackID int, leaf1 string, leaf2 string, count int, builder *Builder, topology *Topology, gridX int, perRow int) {
	//yStep := hostRowGap / hostsPerRow  // Used for staggering the Y coordinate
	x := gridX - (hostsGap*(perRow-1))/2
	y := hostsY

	for i := 1; i <= count; i++ {
		createRackHost(rackID, i, leaf1, leaf2, builder, topology, pos(x, y))
		if i%perRow == 0 {
			x = gridX - (hostsGap*(perRow-1))/2
			y += hostRowGap
		} else {
			x += hostsGap
			//y += yStep
		}
	}
}

// Create a host with one or two NICs connected to the one or two specified switches
func createRackHost(rackID int, hostID int, leaf1 string, leaf2 string, builder *Builder, topology *Topology, pos *GridPosition) *Host {
	nics := make([]NIC, 0, 2)
	nic1 := NIC{
		Mac:  mac(rackID, hostID, 1),
		IPv4: ipv4(rackID, hostID, 1),
		IPV6: ipv6(rackID, hostID, 1),
		Port: builder.NextDevicePortID(leaf1),
	}
	nics = append(nics, nic1)
	if len(leaf2) > 0 {
		nic2 := NIC{
			Mac:  mac(rackID, hostID, 2),
			IPv4: ipv4(rackID, hostID, 2),
			IPV6: ipv6(rackID, hostID, 2),
			Port: builder.NextDevicePortID(leaf2),
		}
		nics = append(nics, nic2)
	}
	host := Host{
		ID:   fmt.Sprintf("host%02d%02d", rackID, hostID),
		NICs: nics,
		Pos:  pos,
	}
	topology.Hosts = append(topology.Hosts, host)
	return &host
}

// Generate a MAC address
func mac(rackID int, hostID int, leafID int) string {
	return fmt.Sprintf("00:ca:fe:%02d:%02d:%02d", rackID, leafID, hostID)
}

// Generate an IPv4 address
func ipv4(rackID int, hostID int, leafID int) string {
	return fmt.Sprintf("10.%d.%d.%d", rackID, leafID, hostID)
}

// Generate an IPv6 address
func ipv6(rackID int, hostID int, leafID int) string {
	return fmt.Sprintf("::ffff:10.10.%d%d.%d", rackID, leafID, hostID)
}

// Generates a coordinate for the i-th element - out of n elements - spaced by a gap and an offset
func coord(i int, n int, gap int, offset int) int {
	return gap*(i-1) - (gap*(n-1))/2 + offset
}

// Generates a grid position for the specified coordinates
func pos(x int, y int) *GridPosition {
	return &GridPosition{X: x, Y: y}
}
