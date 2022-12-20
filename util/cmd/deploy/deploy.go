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

type rootArgs struct {
	configPath       string
	nodePkgPath      string
	nodeBinaryPath   string
	clientPkgPath    string
	clientBinaryPath string
	dbPath           string
	initialPath      string

	nodePkgName   string
	clientPkgName string

	isDebug bool
}

type appPath struct {
	wdPath       string
	binaryPath   string
	configPath   string
	logPath      string
	debugPortNum int
	isDebug      bool
}

const (
	anytypeClientBinaryName = "anytype-client"
	anytypeNodeBinaryName   = "anytype-node"
)

var rootCmd = &cobra.Command{
	Use: "deploy",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		rootArguments := rootArgs{}

		rootArguments.nodePkgName, _ = cmd.Flags().GetString("node-pkg")
		rootArguments.clientPkgName, _ = cmd.Flags().GetString("client-pkg")

		// checking configs
		cfgPath, _ := cmd.Flags().GetString("config-dir")
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			log.With(zap.Error(err)).Fatal("the config directory doesn't exist")
		}
		rootArguments.configPath, _ = filepath.Abs(cfgPath)

		// checking node package
		nodePath, _ := cmd.Flags().GetString("node-path")
		if _, err := os.Stat(path.Join(nodePath, "go.mod")); os.IsNotExist(err) {
			log.With(zap.Error(err)).Fatal("the path to node does not contain a go module")
		}
		rootArguments.nodePkgPath, _ = filepath.Abs(nodePath)

		// checking client package
		clientPath, _ := cmd.Flags().GetString("client-path")
		if _, err := os.Stat(path.Join(clientPath, "go.mod")); os.IsNotExist(err) {
			log.With(zap.Error(err)).Fatal("the path to client does not contain a go module")
		}
		rootArguments.clientPkgPath, _ = filepath.Abs(clientPath)

		// checking binary path
		binaryPath, _ := cmd.Flags().GetString("bin")
		err := createDirectoryIfNotExists(binaryPath)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to create directory")
		}

		absoluteBinPath, _ := filepath.Abs(binaryPath)
		rootArguments.clientBinaryPath = path.Join(absoluteBinPath, anytypeClientBinaryName)
		rootArguments.nodeBinaryPath = path.Join(absoluteBinPath, anytypeNodeBinaryName)

		// getting debug mode
		rootArguments.isDebug, _ = cmd.Flags().GetBool("debug")

		// checking db path
		dbPath, _ := cmd.Flags().GetString("db-path")
		err = createDirectoryIfNotExists(dbPath)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to create directory")
		}
		rootArguments.dbPath, _ = filepath.Abs(dbPath)

		ctx := context.WithValue(context.Background(), "rootArguments", rootArguments)
		cmd.SetContext(ctx)
	},
}

var buildRunAllCmd = &cobra.Command{
	Use:  "build-run-all",
	Long: "build and then run all clients and nodes",
	Run: func(cmd *cobra.Command, args []string) {
		rootArguments, ok := cmd.Context().Value("rootArguments").(rootArgs)
		if !ok {
			log.Fatal("did not get context")
		}

		numNodes, _ := cmd.Flags().GetUint("nodes")
		numClients, _ := cmd.Flags().GetUint("clients")

		// running the script
		err := buildRunAll(rootArguments, numClients, numNodes)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to run the command")
		}
	},
}

var buildAllCmd = &cobra.Command{
	Use:  "build-all",
	Long: "builds both the clients and nodes",
	Run: func(cmd *cobra.Command, args []string) {
		rootArguments, ok := cmd.Context().Value("rootArguments").(rootArgs)
		if !ok {
			log.Fatal("did not get context")
		}

		err := buildAll(rootArguments)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to run the command")
			return
		}
	},
}

var runAllCmd = &cobra.Command{
	Use:  "run-all",
	Long: "runs all clients and nodes",
	Run: func(cmd *cobra.Command, args []string) {
		rootArguments, ok := cmd.Context().Value("rootArguments").(rootArgs)
		if !ok {
			log.Fatal("did not get context")
		}

		numNodes, _ := cmd.Flags().GetUint("nodes")
		numClients, _ := cmd.Flags().GetUint("clients")

		err := runAll(rootArguments, numClients, numNodes)
		if err != nil {
			log.With(zap.Error(err)).Fatal("failed to run the command")
			return
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
	rootCmd.PersistentFlags().Bool("debug", false, "this tells if we should run the profiler")

	buildRunAllCmd.Flags().UintP("nodes", "n", 3, "number of nodes to be generated")
	buildRunAllCmd.Flags().UintP("clients", "c", 2, "number of clients to be generated")
	runAllCmd.Flags().UintP("nodes", "n", 3, "number of nodes to be generated")
	runAllCmd.Flags().UintP("clients", "c", 2, "number of clients to be generated")

	rootCmd.AddCommand(buildRunAllCmd)
	rootCmd.AddCommand(buildAllCmd)
	rootCmd.AddCommand(runAllCmd)
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

func createAppPaths(args rootArgs, binaryPath, appName string, portNum, num int) (appPaths []appPath, err error) {
	appTypePath := path.Join(args.dbPath, appName)
	err = createDirectoryIfNotExists(appTypePath)
	if err != nil {
		return
	}

	for i := 0; i < num; i++ {
		// checking if relevant config exists
		cfgPath := path.Join(args.configPath, fmt.Sprintf("%s%d.yml", appName, i+1))
		if _, err = os.Stat(cfgPath); os.IsNotExist(err) {
			err = fmt.Errorf("not enough %s configs are generated: %w", appName, err)
			return
		}

		// creating directory for each app
		resPath := path.Join(appTypePath, fmt.Sprintf("%d", i+1))
		err = createDirectoryIfNotExists(resPath)
		if err != nil {
			return
		}

		appPaths = append(appPaths, appPath{
			wdPath:       resPath,
			binaryPath:   binaryPath,
			configPath:   cfgPath,
			logPath:      path.Join(resPath, "app.log"),
			debugPortNum: portNum + i + 1,
			isDebug:      args.isDebug,
		})
	}
	return
}

func buildRunAll(args rootArgs, numClients, numNodes uint) (err error) {
	err = buildAll(args)
	if err != nil {
		err = fmt.Errorf("failed to build all: %w", err)
		return
	}

	return runAll(args, numClients, numNodes)
}

func runAll(args rootArgs, numClients uint, numNodes uint) (err error) {
	nodePaths, err := createAppPaths(args, args.nodeBinaryPath, "node", 6060, int(numNodes))
	if err != nil {
		err = fmt.Errorf("failed to create working directories for nodes: %w", err)
		return
	}

	clientPaths, err := createAppPaths(args, args.clientBinaryPath, "client", 6070, int(numClients))
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

func buildAll(args rootArgs) (err error) {
	err = build(args.nodePkgPath, args.nodeBinaryPath, args.nodePkgName)
	if err != nil {
		err = fmt.Errorf("failed to build node: %w", err)
		return
	}

	err = build(args.clientPkgPath, args.clientBinaryPath, args.clientPkgName)
	if err != nil {
		err = fmt.Errorf("failed to build client: %w", err)
		return
	}
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
	log := log
	if app.isDebug {
		log = log.With(zap.String("debug on", fmt.Sprintf("localhost:%d/debug/pprof", app.debugPortNum)))
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("ANYPROF=127.0.0.1:%d", app.debugPortNum))
	}

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
