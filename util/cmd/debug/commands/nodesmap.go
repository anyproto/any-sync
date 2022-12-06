package commands

type NodesMap struct {
	Nodes []struct {
		Addresses    []string `yaml:"grpcAddresses"`
		APIAddresses []string `yaml:"apiAddresses"`
	} `yaml:"nodes"`
	Consensus []struct {
		Addresses []string `yaml:"grpcAddresses"`
	}
	Clients []struct {
		Addresses    []string `yaml:"grpcAddresses"`
		APIAddresses []string `yaml:"apiAddresses"`
	}
}
