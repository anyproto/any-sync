package main

import (
	"context"
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		absolute := absolutePaths{}

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

		err = createDirectoryIfNotExists(binaryPath)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to create directory")
		}
		absoluteBinPath, _ := filepath.Abs(binaryPath)
		absolute.clientBinaryPath = path.Join(absoluteBinPath, anytypeClientBinaryName)
		absolute.nodeBinaryPath = path.Join(absoluteBinPath, anytypeNodeBinaryName)

		// checking db path
		dbPath, err := cmd.Flags().GetString("db-path")
		if err != nil {
			log.With(zap.Error(err)).Fatal("no db-path flag is registered")
		}
		err = createDirectoryIfNotExists(dbPath)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to create directory")
		}
		absolute.dbPath, _ = filepath.Abs(dbPath)

		ctx := context.WithValue(context.Background(), "paths", absolute)
		cmd.SetContext(ctx)
	},
}

var runCmd = &cobra.Command{
	Use: "build-and-run",
	Run: func(cmd *cobra.Command, args []string) {
		paths, ok := cmd.Context().Value("paths").(absolutePaths)
		if !ok {
			log.Fatal("did not get parent context")
		}

		// checking number of nodes to deploy
		numNodes, err := cmd.Flags().GetUint("nodes")
		if err != nil {
			log.With(zap.Error(err)).Fatal("number of nodes is not specified")
		}

		// checking number of clients to deploy
		numClients, err := cmd.Flags().GetUint("clients")
		if err != nil {
			log.With(zap.Error(err)).Fatal("number of clients is not specified")
		}

		// running the script
		err = runAll(paths, numClients, numNodes)

		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to run the command")
		}
	},
}

func init() {
	rootCmd.PersistentFlags().String("config-dir", "etc/configs", "generated configs")
	rootCmd.PersistentFlags().String("node-pkg", "github.com/anytypeio/go-anytype-infrastructure-experiments/node/cmd", "node package")
	rootCmd.PersistentFlags().String("client-pkg", "github.com/anytypeio/go-anytype-infrastructure-experiments/client/cmd", "client package")
	rootCmd.PersistentFlags().String("node-path", "node", "path to node go.mod")
	rootCmd.PersistentFlags().String("client-path", "client", "path to client go.mod")
	rootCmd.PersistentFlags().String("bin", "bin", "path to folder where all the binaries are")
	rootCmd.PersistentFlags().String("db-path", "db", "path to folder where the working directories should be placed")

	runCmd.Flags().UintP("nodes", "n", 3, "number of nodes to be generated")
	runCmd.Flags().UintP("clients", "c", 2, "number of clients to be generated")
	rootCmd.AddCommand(runCmd)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		log.With(zap.Error(err)).Fatal("failed to execute the command")
	}
}

func createDirectoryIfNotExists(dirPath string) (err error) {
	if _, err = os.Stat(dirPath); !os.IsNotExist(err) {
		return
	}
	return os.Mkdir(dirPath, os.ModePerm)
}

func createAppPaths(paths absolutePaths, binaryPath, appName string, num int) (appPaths []appPath, err error) {
	appTypePath := path.Join(paths.dbPath, appName)
	err = createDirectoryIfNotExists(appTypePath)
	if err != nil {
		return
	}

	for i := 0; i < num; i++ {
		// creating directory for each app
		resPath := path.Join(appTypePath, fmt.Sprintf("%d", i+1))
		err = createDirectoryIfNotExists(resPath)
		if err != nil {
			return
		}

		// checking if relevant config exists
		cfgPath := path.Join(paths.configPath, fmt.Sprintf("%s%d.yml", appName, i+1))
		if _, err = os.Stat(cfgPath); os.IsNotExist(err) {
			err = fmt.Errorf("not enough %s configs are generated: %w", appName, err)
			return
		}

		appPaths = append(appPaths, appPath{
			wdPath:     resPath,
			binaryPath: binaryPath,
			configPath: cfgPath,
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

	nodePaths, err := createAppPaths(paths, paths.nodeBinaryPath, "node", int(numNodes))
	if err != nil {
		err = fmt.Errorf("failed to create working directories for nodes: %w", err)
		return
	}

	clientPaths, err := createAppPaths(paths, paths.clientBinaryPath, "client", int(numClients))
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
	return
}

func build(dirPath, binaryPath, packageName string) (err error) {
	cmd := exec.Command("go", "build", "-v", "-o", binaryPath, packageName)
	cmd.Dir = dirPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return
	}
	log.With(zap.String("binary path", binaryPath), zap.String("package name", packageName)).Info("building the app")
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
	log.With(zap.String("working directory", app.wdPath), zap.String("log path", app.logPath)).Info("running the app")
	if err != nil {
		return
	}

	err = cmd.Wait()
	wg.Done()
	return
}
