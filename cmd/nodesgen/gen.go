package main

import (
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"gopkg.in/yaml.v3"
	"math/rand"
	"os"
	"time"
)

var (
	flagAccountConfigFile = flag.String("a", "etc/nodes.yml", "path to account file")
	flagBaseAddress       = flag.String("ba", "127.0.0.1:4430", "base ip for each node (you should change it later)")
	flagNodeCount         = flag.Int("nc", 5, "the count of nodes for which we create the keys")
)

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	if *flagNodeCount <= 0 {
		panic("node count should not be zero or less")
	}

	encryptionDecoder := encryptionkey.NewRSAPrivKeyDecoder()
	signingDecoder := signingkey.NewEDPrivKeyDecoder()

	var nodes []*config.Node
	for i := 0; i < *flagNodeCount; i++ {
		node, err := genRandomNodeKeys(*flagBaseAddress, encryptionDecoder, signingDecoder)
		if err != nil {
			panic(fmt.Sprintf("could not generate keys for node: %v", err))
		}
		nodes = append(nodes, node)
	}
	nodeInfo := config.NodeInfo{
		CurrentAlias: nodes[0].Alias,
		Nodes:        nodes,
	}
	bytes, err := yaml.Marshal(nodeInfo)
	if err != nil {
		panic(fmt.Sprintf("could not marshal the keys: %v", err))
	}

	err = os.WriteFile(*flagAccountConfigFile, bytes, 0644)
	if err != nil {
		panic(fmt.Sprintf("could not write the generated nodes to file: %v", err))
	}
}

func genRandomNodeKeys(address string, encKeyDecoder keys.Decoder, signKeyDecoder keys.Decoder) (*config.Node, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return nil, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nil, err
	}

	encEncKey, err := encKeyDecoder.EncodeToString(encKey)
	if err != nil {
		return nil, err
	}

	encSignKey, err := signKeyDecoder.EncodeToString(signKey)
	if err != nil {
		return nil, err
	}

	return &config.Node{
		Alias:         randString(5),
		Address:       address,
		SigningKey:    encSignKey,
		EncryptionKey: encEncKey,
	}, nil
}

func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := 0; i < n; i++ {
		idx := rand.Intn(len(letterBytes))
		b[i] = letterBytes[idx]
	}

	return string(b)
}
