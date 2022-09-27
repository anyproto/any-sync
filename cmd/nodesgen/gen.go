package main

import (
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/peer"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

var (
	flagNodeMap = flag.String("n", "cmd/nodesgen/nodemap.yml", "path to nodes map file")
	flagEtcPath = flag.String("e", "etc", "path to etc directory")
)

type NodesMap struct {
	Nodes []struct {
		Addresses []string `yaml:"grpcAddresses"`
		APIPort   string   `yaml:"apiPort"`
	} `yaml:"nodes"`
}

func main() {
	nodesMap := &NodesMap{}
	data, err := ioutil.ReadFile(*flagNodeMap)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, nodesMap)
	if err != nil {
		panic(err)
	}
	flag.Parse()

	var configs []config.Config
	var nodes []config.Node
	for _, n := range nodesMap.Nodes {
		cfg, err := genConfig(n.Addresses, n.APIPort)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		configs = append(configs, cfg)

		node := config.Node{
			PeerId:        cfg.Account.PeerId,
			Address:       cfg.GrpcServer.ListenAddrs[0],
			SigningKey:    cfg.Account.SigningKey,
			EncryptionKey: cfg.Account.EncryptionKey,
		}
		nodes = append(nodes, node)
	}
	for idx := range configs {
		configs[idx].Nodes = nodes
	}

	// saving configs
	configsPath := fmt.Sprintf("%s/configs", *flagEtcPath)
	createDir := func() {
		err := os.Mkdir(configsPath, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("failed to create the configs directory: %v", err))
		}
	}
	if _, err := os.Stat(configsPath); os.IsNotExist(err) {
		createDir()
	} else {
		err = os.RemoveAll(configsPath)
		if err != nil {
			panic(fmt.Sprintf("failed to remove the configs directory: %v", err))
		}
		createDir()
	}
	for idx, cfg := range configs {
		path := fmt.Sprintf("%s/config%d.yml", configsPath, idx+1)
		bytes, err := yaml.Marshal(cfg)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(path, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}
	}
}

func genConfig(addresses []string, apiPort string) (config.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return config.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return config.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return config.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return config.Config{}, err
	}

	peerID, err := peer.IDFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return config.Config{}, err
	}

	return config.Config{
		Anytype: config.Anytype{SwarmKey: "/key/swarm/psk/1.0.0/base16/209992e611c27d5dce8fbd2e7389f6b51da9bee980992ef60739460b536139ec"},
		GrpcServer: config.GrpcServer{
			ListenAddrs: addresses,
			TLS:         false,
		},
		Account: config.Account{
			PeerId:        peerID.String(),
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		APIServer: config.APIServer{
			Port: apiPort,
		},
		Space: config.Space{
			GCTTL:      60,
			SyncPeriod: 10,
		},
	}, nil
}
