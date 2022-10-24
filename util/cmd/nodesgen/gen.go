package main

import (
	"flag"
	"fmt"
	config2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/peer"
	cconfig "github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/config"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

var (
	flagNodeMap = flag.String("n", "util/cmd/nodesgen/nodemap.yml", "path to nodes map file")
	flagEtcPath = flag.String("e", "etc", "path to etc directory")
)

type NodesMap struct {
	Nodes []struct {
		Addresses []string `yaml:"grpcAddresses"`
		APIPort   string   `yaml:"apiPort"`
	} `yaml:"nodes"`
	Consensus []struct {
		Addresses []string `yaml:"grpcAddresses"`
	}
	Clients []struct {
		Addresses []string `yaml:"grpcAddresses"`
		APIPort   string   `yaml:"apiPort"`
	}
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

	var configs []config2.Config
	var nodes []config2.Node
	for _, n := range nodesMap.Nodes {
		cfg, err := genNodeConfig(n.Addresses, n.APIPort)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		configs = append(configs, cfg)

		node := config2.Node{
			PeerId:        cfg.Account.PeerId,
			Address:       cfg.GrpcServer.ListenAddrs[0],
			SigningKey:    cfg.Account.SigningKey,
			EncryptionKey: cfg.Account.EncryptionKey,
		}
		nodes = append(nodes, node)
	}

	var clientConfigs []config2.Config
	for _, c := range nodesMap.Clients {
		cfg, err := genClientConfig(c.Addresses, c.APIPort)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		clientConfigs = append(clientConfigs, cfg)
	}

	var consConfigs []cconfig.Config
	for _, n := range nodesMap.Consensus {
		cfg, err := genConsensusConfig(n.Addresses)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		consConfigs = append(consConfigs, cfg)
	}
	for idx := range configs {
		configs[idx].Nodes = nodes
	}
	for idx := range clientConfigs {
		clientConfigs[idx].Nodes = nodes
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
		path := fmt.Sprintf("%s/node%d.yml", configsPath, idx+1)
		bytes, err := yaml.Marshal(cfg)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(path, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}
	}
	for idx, cfg := range clientConfigs {
		path := fmt.Sprintf("%s/client%d.yml", configsPath, idx+1)
		bytes, err := yaml.Marshal(cfg)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(path, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}
	}
	for idx, cfg := range consConfigs {
		path := fmt.Sprintf("%s/cons%d.yml", configsPath, idx+1)
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

func genNodeConfig(addresses []string, apiPort string) (config2.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return config2.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return config2.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return config2.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return config2.Config{}, err
	}

	peerID, err := peer.IDFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return config2.Config{}, err
	}

	return config2.Config{
		Anytype: config2.Anytype{SwarmKey: "/key/swarm/psk/1.0.0/base16/209992e611c27d5dce8fbd2e7389f6b51da9bee980992ef60739460b536139ec"},
		GrpcServer: config2.GrpcServer{
			ListenAddrs: addresses,
			TLS:         false,
		},
		Storage: config2.Storage{Path: "db"},
		Account: config2.Account{
			PeerId:        peerID.String(),
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		APIServer: config2.APIServer{
			Port: apiPort,
		},
		Space: config2.Space{
			GCTTL:      60,
			SyncPeriod: 10,
		},
	}, nil
}

func genClientConfig(addresses []string, apiPort string) (config2.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return config2.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return config2.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return config2.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return config2.Config{}, err
	}

	peerID, err := peer.IDFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return config2.Config{}, err
	}

	return config2.Config{
		Anytype: config2.Anytype{SwarmKey: "/key/swarm/psk/1.0.0/base16/209992e611c27d5dce8fbd2e7389f6b51da9bee980992ef60739460b536139ec"},
		GrpcServer: config2.GrpcServer{
			ListenAddrs: addresses,
			TLS:         false,
		},
		Storage: config2.Storage{Path: "db"},
		Account: config2.Account{
			PeerId:        peerID.String(),
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		APIServer: config2.APIServer{
			Port: apiPort,
		},
		Space: config2.Space{
			GCTTL:      60,
			SyncPeriod: 10,
		},
	}, nil
}

func genConsensusConfig(addresses []string) (cconfig.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return cconfig.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return cconfig.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return cconfig.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return cconfig.Config{}, err
	}

	peerID, err := peer.IDFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return cconfig.Config{}, err
	}

	return cconfig.Config{
		GrpcServer: config2.GrpcServer{
			ListenAddrs: addresses,
			TLS:         false,
		},
		Account: config2.Account{
			PeerId:        peerID.String(),
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		Mongo: cconfig.Mongo{
			Connect:       "mongodb://localhost:27017/?w=majority",
			Database:      "consensus",
			LogCollection: "log",
		},
	}, nil
}
