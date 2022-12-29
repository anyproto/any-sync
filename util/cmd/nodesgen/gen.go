package main

import (
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	clconfig "github.com/anytypeio/go-anytype-infrastructure-experiments/client/config"
	commonaccount "github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/peer"
	consconfig "github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/config"
	fconfig "github.com/anytypeio/go-anytype-infrastructure-experiments/filenode/config"
	config "github.com/anytypeio/go-anytype-infrastructure-experiments/node/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/storage"
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
		Addresses    []string `yaml:"grpcAddresses"`
		APIAddresses []string `yaml:"apiAddresses"`
	} `yaml:"nodes"`
	Consensus []struct {
		Addresses []string `yaml:"grpcAddresses"`
	}
	FileNodes []struct {
		Addresses []string `yaml:"grpcAddresses"`
	} `yaml:"fileNodes""`
	Clients []struct {
		Addresses    []string `yaml:"grpcAddresses"`
		APIAddresses []string `yaml:"apiAddresses"`
	}
}

func main() {
	flag.Parse()
	nodesMap := &NodesMap{}
	data, err := ioutil.ReadFile(*flagNodeMap)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, nodesMap)
	if err != nil {
		panic(err)
	}

	var configs []config.Config
	var nodes []nodeconf.NodeConfig
	for i, n := range nodesMap.Nodes {
		cfg, err := genNodeConfig(n.Addresses, n.APIAddresses, i+1)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		configs = append(configs, cfg)

		node := nodeconf.NodeConfig{
			PeerId:        cfg.Account.PeerId,
			Addresses:     cfg.GrpcServer.Server.ListenAddrs,
			SigningKey:    cfg.Account.SigningKey,
			EncryptionKey: cfg.Account.EncryptionKey,
			Types:         []nodeconf.NodeType{nodeconf.NodeTypeTree},
		}
		nodes = append(nodes, node)
	}

	encClientKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		panic(fmt.Sprintf("could not generate client encryption key: %s", err.Error()))
	}

	signClientKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		panic(fmt.Sprintf("could not generate client signing key: %s", err.Error()))
	}

	var clientConfigs []clconfig.Config
	for i, c := range nodesMap.Clients {
		cfg, err := genClientConfig(c.Addresses, c.APIAddresses, encClientKey, signClientKey, i+1)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		clientConfigs = append(clientConfigs, cfg)
	}

	var consConfigs []consconfig.Config
	for _, n := range nodesMap.Consensus {
		cfg, err := genConsensusConfig(n.Addresses)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		consConfigs = append(consConfigs, cfg)
		nodes = append(nodes, nodeconf.NodeConfig{
			PeerId:    cfg.Account.PeerId,
			Addresses: n.Addresses,
			Types:     []nodeconf.NodeType{nodeconf.NodeTypeConsensus},
		})
	}
	var fileConfigs []fconfig.Config
	for i, n := range nodesMap.FileNodes {
		cfg, err := getFileNodeConfig(n.Addresses, i+1)
		if err != nil {
			panic(fmt.Sprintf("could not generate the config file: %s", err.Error()))
		}
		fileConfigs = append(fileConfigs, cfg)
		nodes = append(nodes, nodeconf.NodeConfig{
			PeerId:    cfg.Account.PeerId,
			Addresses: n.Addresses,
			Types:     []nodeconf.NodeType{nodeconf.NodeTypeFile},
		})
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
	for idx, cfg := range fileConfigs {
		path := fmt.Sprintf("%s/file%d.yml", configsPath, idx+1)
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

func genNodeConfig(addresses []string, apiAddresses []string, num int) (config.Config, error) {
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

	peerID, err := peer.IdFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return config.Config{}, err
	}

	return config.Config{
		GrpcServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: addresses,
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Storage: storage.Config{Path: fmt.Sprintf("db/node/%d/data", num)},
		Account: commonaccount.Config{
			PeerId:        peerID.String(),
			PeerKey:       encSignKey,
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		APIServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: apiAddresses,
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Space: commonspace.Config{
			GCTTL:      60,
			SyncPeriod: 600,
		},
	}, nil
}

func genClientConfig(addresses []string, apiAddresses []string, encKey encryptionkey.PrivKey, signKey signingkey.PrivKey, num int) (clconfig.Config, error) {
	peerKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return clconfig.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return clconfig.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return clconfig.Config{}, err
	}

	encPeerKey, err := keys.EncodeKeyToString(peerKey)
	if err != nil {
		return clconfig.Config{}, err
	}

	peerID, err := peer.IdFromSigningPubKey(peerKey.GetPublic())
	if err != nil {
		return clconfig.Config{}, err
	}

	return clconfig.Config{
		GrpcServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: addresses,
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Storage: badgerprovider.Config{Path: fmt.Sprintf("db/client/%d", num)},
		Account: commonaccount.Config{
			PeerId:        peerID.String(),
			PeerKey:       encPeerKey,
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		APIServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: apiAddresses,
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Space: commonspace.Config{
			GCTTL:      60,
			SyncPeriod: 20,
		},
	}, nil
}

func genConsensusConfig(addresses []string) (consconfig.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return consconfig.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return consconfig.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return consconfig.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return consconfig.Config{}, err
	}

	peerID, err := peer.IdFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return consconfig.Config{}, err
	}

	return consconfig.Config{
		GrpcServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: addresses,
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Account: commonaccount.Config{
			PeerId:        peerID.String(),
			PeerKey:       encSignKey,
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		Mongo: consconfig.Mongo{
			Connect:       "mongodb://localhost:27017/?w=majority",
			Database:      "consensus",
			LogCollection: "log",
		},
	}, nil
}

func getFileNodeConfig(addresses []string, num int) (fconfig.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return fconfig.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return fconfig.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encKey)
	if err != nil {
		return fconfig.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey)
	if err != nil {
		return fconfig.Config{}, err
	}

	peerID, err := peer.IdFromSigningPubKey(signKey.GetPublic())
	if err != nil {
		return fconfig.Config{}, err
	}
	return fconfig.Config{
		GrpcServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: addresses,
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Account: commonaccount.Config{
			PeerId:        peerID.String(),
			PeerKey:       encSignKey,
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		FileStorePogreb: fconfig.FileStorePogreb{
			Path: fmt.Sprintf("db/file/%d", num),
		},
	}, nil
}
