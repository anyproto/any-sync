package main

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
)

var log = logger.NewNamed("cmd.deploy")

type absolutePaths struct {
	configPath       string
	nodePkgPath      string
	nodeBinaryPath   string
	clientPkgPath    string
	clientBinaryPath string
	dbPath           string
	initialPath      string

	nodePkgName   string
	clientPkgName string
}

type appPath struct {
	wdPath     string
	binaryPath string
	configPath string
	logPath    string
}

const (
	anytypeClientBinaryName = "anytype-client"
	anytypeNodeBinaryName   = "anytype-node"
)

var rootCmd = &cobra.Command{
	Use: "deploy",
	Run: func(cmd *cobra.Command, args []string) {
		absolute := absolutePaths{}
		// saving working directory
		wd, _ := os.Getwd()
		absolute.initialPath, _ = filepath.Abs(wd)

		// checking number of nodes to deploy
		numNodes, err := cmd.Flags().GetUint("n")
		if err != nil {
			log.With(zap.Error(err)).Fatal("number of nodes is not specified")
		}

		// checking number of clients to deploy
		numClients, err := cmd.Flags().GetUint("c")
		if err != nil {
			log.With(zap.Error(err)).Fatal("number of clients is not specified")
		}

		// checking package names
		nodePkgName, err := cmd.Flags().GetString("node-pkg")
		if err != nil {
			log.With(zap.Error(err)).Fatal("node package is not specified")
		}
		absolute.nodePkgName = nodePkgName

		clientPkgName, err := cmd.Flags().GetString("client-pkg")
		if err != nil {
			log.With(zap.Error(err)).Fatal("client package is not specified")
		}
		absolute.clientPkgName = clientPkgName

		// checking configs
		cfgPath, err := cmd.Flags().GetString("config-dir")
		if err != nil {
			log.With(zap.Error(err)).Fatal("no config-dir flag is registered")
		}

		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			log.With(zap.Error(err)).Fatal("the config directory doesn't exist")
		}
		absolute.configPath, _ = filepath.Abs(cfgPath)

		// check if all configs really exist for nodes and clients to be deployed
		for i := 0; i < int(numNodes); i++ {
			configName := fmt.Sprintf("node%d.yml", i+1)
			configPath := path.Join(absolute.configPath, configName)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				log.With(zap.Error(err)).Fatal("not enough node configs are generated")
			}
		}
		for i := 0; i < int(numClients); i++ {
			configName := fmt.Sprintf("client%d.yml", i+1)
			configPath := path.Join(absolute.configPath, configName)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				log.With(zap.Error(err)).Fatal("not enough client configs are generated")
			}
		}

		// checking node package
		nodePath, err := cmd.Flags().GetString("node-path")
		if err != nil {
			log.With(zap.Error(err)).Fatal("no node-path flag is registered")
		}

		if _, err := os.Stat(path.Join(nodePath, "go.mod")); os.IsNotExist(err) {
			log.With(zap.Error(err)).Fatal("the path to node does not contain a go module")
		}
		absolute.nodePkgPath, _ = filepath.Abs(nodePath)

		// checking client package
		clientPath, err := cmd.Flags().GetString("client-path")
		if err != nil {
			log.With(zap.Error(err)).Fatal("no client-path flag is registered")
		}

		if _, err := os.Stat(path.Join(clientPath, "go.mod")); os.IsNotExist(err) {
			log.With(zap.Error(err)).Fatal("the path to client does not contain a go module")
		}
		absolute.clientPkgPath, _ = filepath.Abs(clientPath)

		// checking binary path
		binaryPath, err := cmd.Flags().GetString("bin")
		if err != nil {
			log.With(zap.Error(err)).Fatal("no bin flag is registered")
		}

		createDirectoryIfNotExists(binaryPath)
		absoluteBinPath, _ := filepath.Abs(binaryPath)
		absolute.clientBinaryPath = path.Join(absoluteBinPath, anytypeClientBinaryName)
		absolute.nodeBinaryPath = path.Join(absoluteBinPath, anytypeNodeBinaryName)

		// checking db path
		dbPath, err := cmd.Flags().GetString("db-path")
		if err != nil {
			log.With(zap.Error(err)).Fatal("no db-path flag is registered")
		}
		createDirectoryIfNotExists(dbPath)
		absolute.dbPath, _ = filepath.Abs(dbPath)

		// running the script
		err = runAll(absolute, numClients, numNodes)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to run the command")
		}
	},
}

func init() {
	rootCmd.Flags().String("config-dir", "etc/configs", "generated configs")
	rootCmd.Flags().Uint("n", 3, "number of nodes to be generated")
	rootCmd.Flags().Uint("c", 2, "number of clients to be generated")
	rootCmd.Flags().String("node-pkg", "github.com/anytypeio/go-anytype-infrastructure-experiments/node/cmd", "node package")
	rootCmd.Flags().String("client-pkg", "github.com/anytypeio/go-anytype-infrastructure-experiments/client/cmd", "client package")
	rootCmd.Flags().String("node-path", "node", "path to node go.mod")
	rootCmd.Flags().String("client-path", "client", "path to client go.mod")
	rootCmd.Flags().String("bin", "bin", "path to folder where all the binaries are")
	rootCmd.Flags().String("db-path", "db", "path to folder where the working directories should be placed")
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		log.With(zap.Error(err)).Fatal("failed to execute the command")
	}
}

func createDirectoryIfNotExists(dirPath string) {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		return
	}
	err := os.Mkdir(dirPath, os.ModePerm)
	if err != nil {
		log.With(zap.Error(err)).Fatal("failed to create directory")
	}
}

func createNodePaths(paths absolutePaths, num int) (appPaths []appPath, err error) {
	prefixPath := path.Join(paths.dbPath, "node")
	createDirectoryIfNotExists(prefixPath)
	for i := 0; i < num; i++ {
		resPath := path.Join(prefixPath, fmt.Sprintf("%d", i+1))
		createDirectoryIfNotExists(resPath)
		appPaths = append(appPaths, appPath{
			wdPath:     resPath,
			binaryPath: paths.nodeBinaryPath,
			configPath: path.Join(paths.configPath, fmt.Sprintf("node%d.yml", i+1)),
			logPath:    path.Join(resPath, "app.log"),
		})
	}
	return
}

func createClientPaths(paths absolutePaths, num int) (appPaths []appPath, err error) {
	prefixPath := path.Join(paths.dbPath, "client")
	createDirectoryIfNotExists(prefixPath)
	for i := 0; i < num; i++ {
		resPath := path.Join(prefixPath, fmt.Sprintf("%d", i+1))
		createDirectoryIfNotExists(resPath)
		appPaths = append(appPaths, appPath{
			wdPath:     resPath,
			binaryPath: paths.clientBinaryPath,
			configPath: path.Join(paths.configPath, fmt.Sprintf("client%d.yml", i+1)),
			logPath:    path.Join(resPath, "app.log"),
		})
	}
	return
}

func runAll(paths absolutePaths, numClients, numNodes uint) (err error) {
	err = build(paths.nodePkgPath, paths.nodeBinaryPath, paths.nodePkgName)
	if err != nil {
		err = fmt.Errorf("failed to build node: %w", err)
		return
	}

	err = build(paths.clientPkgPath, paths.clientBinaryPath, paths.clientPkgName)
	if err != nil {
		err = fmt.Errorf("failed to build client: %w", err)
		return
	}

	nodePaths, err := createNodePaths(paths, int(numNodes))
	if err != nil {
		err = fmt.Errorf("failed to create working directories for nodes: %w", err)
		return
	}

	clientPaths, err := createClientPaths(paths, int(numClients))
	if err != nil {
		err = fmt.Errorf("failed to create working directories for clients: %w", err)
		return
	}
	wg := sync.WaitGroup{}
	for _, nodePath := range nodePaths {
		wg.Add(1)
		go func(path appPath) {
			err = runApp(path, &wg)
			if err != nil {
				log.With(zap.Error(err)).Error("running node failed with error")
			}
		}(nodePath)
	}
	for _, clientPath := range clientPaths {
		wg.Add(1)
		go func(path appPath) {
			err = runApp(path, &wg)
			if err != nil {
				log.With(zap.Error(err)).Error("running node failed with error")
			}
		}(clientPath)
	}
	wg.Wait()
	log.Info("running finished")
	return
}

func build(dirPath, binaryPath, packageName string) (err error) {
	fmt.Println(dirPath, binaryPath, packageName)
	cmd := exec.Command("go", "build", "-v", "-o", binaryPath, packageName)
	cmd.Dir = dirPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return
	}
	return cmd.Wait()
}

func runApp(app appPath, wg *sync.WaitGroup) (err error) {
	cmd := exec.Command(app.binaryPath, "-c", app.configPath)
	cmd.Dir = app.wdPath
	file, err := os.OpenFile(app.logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		return
	}
	defer file.Close()
	cmd.Stdout = file
	cmd.Stderr = file
	err = cmd.Start()
	log.With(zap.String("working directory", app.wdPath)).Info("run the app")
	if err != nil {
		return
	}

	err = cmd.Wait()
	wg.Done()
	return
}
