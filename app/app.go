package app

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var (
	// values of this vars will be defined while compilation
	AppName, GitCommit, GitBranch, GitState, GitSummary, BuildDate string
	name                                                           string
)

var (
	log               = logger.NewNamed("app")
	StopDeadline      = time.Minute
	StopWarningAfter  = time.Second * 10
	StartWarningAfter = time.Second * 10
)

// Component is a minimal interface for a common app.Component
type Component interface {
	// Init will be called first
	// When returned error is not nil - app start will be aborted
	Init(a *App) (err error)
	// Name must return unique service name
	Name() (name string)
}

type AppModule interface {
	Component
	Inject(a *App)
}

// ComponentRunnable is an interface for realizing ability to start background processes or deep configure service
type ComponentRunnable interface {
	Component
	// Run will be called after init stage
	// Non-nil error also will be aborted app start
	Run(ctx context.Context) (err error)
	// Close will be called when app shutting down
	// Also will be called when service return error on Init or Run stage
	// Non-nil error will be printed to log
	Close(ctx context.Context) (err error)
}

type ComponentStatable interface {
	StateChange(state int)
}

// App is the central part of the application
// It contains and manages all components
type App struct {
	parent            *App
	isChild           bool
	components        []Component
	privateComponents []Component
	scopes            []*App
	mu                sync.RWMutex
	startStat         Stat
	stopStat          Stat
	deviceState       int
	versionName       string
	anySyncVersion    string
	Order             map[string][]string
	collect           bool
}

func (app *App) getOrder() map[string][]string {
	current := app
	for {
		if current.parent == nil {
			break
		}
		current = current.parent
	}
	if current.Order == nil {
		current.Order = make(map[string][]string)
	}
	return current.Order
}

func (app *App) getTop() *App {
	current := app
	for {
		if current.parent == nil {
			break
		}
		current = current.parent
	}
	return current
}

func (app *App) isCollect() bool {
	current := app
	for {
		if current.parent == nil {
			break
		}
		current = current.parent
	}
	return current.collect
}

// Name returns app name
func (app *App) Name() string {
	return name
}

func (app *App) AppName() string {
	return AppName
}

// Version return app version
func (app *App) Version() string {
	return GitSummary
}

// SetVersionName sets the custom application version
func (app *App) SetVersionName(v string) {
	app.versionName = v
}

// VersionName returns a string with the settled app version or auto-generated version if it didn't set
func (app *App) VersionName() string {
	if app.versionName != "" {
		return app.versionName
	}
	return AppName + ":" + GitSummary + "/any-sync:" + app.AnySyncVersion()
}

type Stat struct {
	SpentMsPerComp map[string]int64
	SpentMsTotal   int64
}

// StartStat returns total time spent per comp for the last Start
func (app *App) StartStat() Stat {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.startStat
}

// StopStat returns total time spent per comp for the last Close
func (app *App) StopStat() Stat {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.stopStat
}

// VersionDescription return the full info about the build
func (app *App) VersionDescription() string {
	return VersionDescription()
}

func Version() string {
	return GitSummary
}

func VersionDescription() string {
	return fmt.Sprintf("build on %s from %s at #%s(%s)", BuildDate, GitBranch, GitCommit, GitState)
}

// ChildApp creates a child container which has access to parent's components
// It doesn't call Start on any of the parent's components
func (app *App) ChildApp() *App {
	return &App{
		parent:         app,
		isChild:        true,
		deviceState:    app.deviceState,
		anySyncVersion: app.AnySyncVersion(),
	}
}

// Register adds service to registry
// All components will be started in the Order they were registered
func (app *App) RegisterScope(scope *App) *App {
	app.mu.Lock()
	defer app.mu.Unlock()
	scope.parent = app
	app.scopes = append(app.scopes, scope)
	return app
}

// Register adds service to registry
// All components will be started in the order they were registered
func (app *App) Register(s Component) *App {
	app.mu.Lock()
	defer app.mu.Unlock()
	for _, es := range app.components {
		if s.Name() == es.Name() {
			panic(fmt.Errorf("component '%s' already registered", s.Name()))
		}
	}
	app.components = append(app.components, s)
	return app
}

// Component returns service by name
// If service with given name wasn't registered, nil will be returned
func (app *App) Component(name string) Component {
	app.mu.RLock()
	defer app.mu.RUnlock()

	for _, s := range app.privateComponents {
		if s.Name() == name {
			return s
		}
	}

	for _, s := range app.components {
		if s.Name() == name {
			return s
		}
	}

	if app.isChild {
		result := app.getPublicComponentInParent(name)
		if result != nil {
			return result
		}
	}

	return app.getTop().getPublicComponentInScopes(name)
}

func (app *App) getPublicComponentInParent(name string) Component {
	current := app
	for _, s := range current.components {
		if s.Name() == name {
			return s
		}
	}
	current = current.parent
	if current == nil {
		return nil
	}
	return current.getPublicComponentInParent(name)
}

func (app *App) getPublicComponentInScopes(name string) Component {
	current := app
	for _, s := range current.components {
		if s.Name() == name {
			return s
		}
	}
	for _, s := range current.scopes {
		component := s.getPublicComponentInScopes(name)
		if component != nil {
			return component
		}
	}
	return nil
}

// MustComponent is like Component, but it will panic if service wasn't found
func (app *App) MustComponent(name string) Component {
	s := app.Component(name)
	app.getPair(s)
	if s == nil {
		panic(fmt.Errorf("component '%s' not registered", name))
	}
	return s
}

func (app *App) getPair(found any) {
	if !app.isCollect() {
		return
	}
	// if app.Order == nil {
	//	app.Order = make(map[string][]string)
	// }
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		fullTypeName := reflect.TypeOf(found).PkgPath() + "." + reflect.TypeOf(found).Name()
		if len(fullTypeName) == 1 {
			fullTypeName = reflect.TypeOf(found).Elem().PkgPath() + "." + reflect.TypeOf(found).Elem().Name()
		}
		calledFrom := runtime.FuncForPC(details.Entry()).Name()
		calledForType := fullTypeName

		// Убираем префикс "go-di-proposal/" из путей
		calledFrom = strings.TrimPrefix(calledFrom, "github.com/anyproto/")
		calledForType = strings.TrimPrefix(calledForType, "github.com/anyproto/")

		// Убираем часть после ".(*"
		if strings.Contains(calledFrom, ".(*") {
			calledFrom = strings.Split(calledFrom, ".(*")[0] + "." + strings.Split(strings.Split(calledFrom, ".(*")[1], ").")[0]
		}
		calledForType = strings.Split(calledForType, ".(*")[0]
		app.getOrder()[calledFrom] = append(app.getOrder()[calledFrom], calledForType)
	}
}

// MustComponent - generic version of app.MustComponent
func MustComponent[T any](app *App) T {
	app.mu.RLock()
	defer app.mu.RUnlock()
	current := app
	for _, s := range current.privateComponents {
		if v, ok := s.(T); ok {
			app.getPair(v)
			return v
		}
	}

	if app.isChild {
		result, err := getPublicComponentInParent[T](app)
		if err == nil {
			app.getPair(result)
			return result
		}
	}

	result, err := getPublicComponentInScopes[T](app.getTop())
	if err != nil {
		empty := new(T)
		panic(fmt.Errorf("component with interface %T is not found", empty))
	}
	app.getPair(result)
	return result
}

func getPublicComponentInParent[i any](current *App) (i, error) {
	for _, s := range current.components {
		if v, ok := s.(i); ok {
			return v, nil
		}
	}
	current = current.parent
	if current == nil {
		return *new(i), errors.New("can't find the component")
	}
	return getPublicComponentInParent[i](current)
}

func getPublicComponentInScopes[i any](app *App) (i, error) {
	for _, s := range app.components {
		if v, ok := s.(i); ok {
			return v, nil
		}
	}
	for _, s := range app.scopes {
		component, _ := getPublicComponentInScopes[i](s)
		if v, ok := any(component).(i); ok {
			return v, nil
		}
	}
	return *new(i), errors.New("can't find the component")
}

// ComponentNames returns all registered names
func (app *App) ComponentNames() (names []string) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	names = make([]string, 0, len(app.components))
	current := app
	for current != nil {
		for _, c := range current.components {
			names = append(names, c.Name())
		}
		current = current.parent
	}
	return
}

func (app *App) RegisterPrivate(s Component) *App {
	app.mu.Lock()
	defer app.mu.Unlock()
	for _, es := range app.components {
		if s.Name() == es.Name() {
			panic(fmt.Errorf("component '%s' already registered", s.Name()))
		}
	}
	app.privateComponents = append(app.privateComponents, s)
	return app
}

// Start starts the application
// All registered services will be initialized and started
func (app *App) Start(ctx context.Context) (err error) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	// app.startStat.SpentMsPerComp = make(map[string]int64) todo
	done := make(chan struct{})
	// todo fix closing
	// closeServices := func(idx int) {
	//	for i := idx; i >= 0; i-- {
	//		if serviceClose, ok := app.components[i].(ComponentRunnable); ok {
	//			if e := serviceClose.Close(ctx); e != nil {
	//			}
	//		}
	//	}
	//	for i := len(app.privateComponents) - 1; i >= 0; i-- {
	//		if serviceClose, ok := app.privateComponents[i].(ComponentRunnable); ok {
	//			if e := serviceClose.Close(ctx); e != nil {
	//			}
	//		}
	//	}
	//	for i := len(app.scopes) - 1; i >= 0; i-- {
	//		if e := app.scopes[i].Close(ctx); e != nil {
	//		}
	//	}
	// }

	if app.isChild == false {
		app.collect = true
	}

	if app.parent == nil {
		app.injectModules()
	}
	app.init() // todo error handling

	// for j, s := range app.scopes {
	//	if err = s.Start(ctx); err != nil {
	//		for i := j; i >= 0; i-- {
	//			app.scopes[j].Close(ctx)
	//		}
	//		return fmt.Errorf("can't start scope '%s': %w", s.Name(), err)
	//	}
	// }

	app.collect = false

	app.runScopes(ctx) // todo error handling

	close(done)
	if app.startStat.SpentMsTotal > StartWarningAfter.Milliseconds() {
	}
	return
}

// IterateComponents iterates over all registered components. It's safe for concurrent use.
func (app *App) IterateComponents(fn func(Component)) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	for _, s := range app.components {
		fn(s)
	}
}

func stackAllGoroutines() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

func statLogger(stat Stat, ctxLogger logger.CtxLogger) logger.CtxLogger {
	l := ctxLogger
	// for k, v := range stat.SpentMsPerComp {
	// 	l = l.With(zap.Int64(k, v))
	// }
	l = l.With(zap.Int64("total", stat.SpentMsTotal))

	return l
}

// Close stops the application
// All components with ComponentRunnable implementation will be closed in the reversed order
func (app *App) Close(ctx context.Context) error {
	log.Debug("close components...")
	app.mu.RLock()
	defer app.mu.RUnlock()
	app.stopStat.SpentMsPerComp = make(map[string]int64)
	var currentComponentStopping string
	done := make(chan struct{})

	go func() {
		select {
		case <-done:
			return
		case <-time.After(StopWarningAfter):
			statLogger(app.stopStat, log).
				With(zap.String("in_progress", currentComponentStopping)).
				Warn("components close in progress")
		}
	}()
	go func() {
		select {
		case <-done:
			return
		case <-time.After(StopDeadline):
			_, _ = os.Stderr.Write([]byte("app.Close timeout\n"))
			_, _ = os.Stderr.Write(stackAllGoroutines())
			panic("app.Close timeout")
		}
	}()

	var errs []string
	for i := len(app.components) - 1; i >= 0; i-- {
		if serviceClose, ok := app.components[i].(ComponentRunnable); ok {
			start := time.Now()
			currentComponentStopping = app.components[i].Name()
			if e := serviceClose.Close(ctx); e != nil {
				errs = append(errs, fmt.Sprintf("Component '%s' close error: %v", serviceClose.Name(), e))
			}
			spent := time.Since(start).Milliseconds()
			app.stopStat.SpentMsTotal += spent
			// app.stopStat.SpentMsPerComp[app.components[i].Name()] = spent todo restore
		}
	}
	close(done)
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	l := statLogger(app.stopStat, log)
	if app.stopStat.SpentMsTotal > StopWarningAfter.Milliseconds() {
		l.Warn("all components have been closed")
	}

	l.Debug("all components have been closed")
	return nil
}

func (app *App) SetDeviceState(state int) {
	if app == nil {
		return
	}
	app.mu.RLock()
	defer app.mu.RUnlock()
	app.deviceState = state
	for _, component := range app.components {
		if statable, ok := component.(ComponentStatable); ok {
			statable.StateChange(state)
		}
	}
}

var onceVersion sync.Once

func (app *App) AnySyncVersion() string {
	onceVersion.Do(func() {
		info, ok := debug.ReadBuildInfo()
		if ok {
			for _, mod := range info.Deps {
				if mod.Path == "github.com/anyproto/any-sync" {
					app.anySyncVersion = mod.Version
					break
				}
			}
		}
	})
	return app.anySyncVersion
}

// Функция для поиска циклов в графе
func findCycles(graph map[string][]string) ([][]string, bool) {
	cycles := make(map[int][]string)

	for node := range graph {
		dfs(graph, node, make(map[string]struct{}), []string{}, cycles)
	}
	return convertMapToSlice(cycles), len(cycles) != 0
}

// Функция для удаления всех элементов в слайсе перед указанной строкой
func removeElementsBefore(slice []string, element string) []string {
	for i, val := range slice {
		if val == element {
			// Создание нового слайса с копией элементов
			newSlice := make([]string, len(slice[i:]))
			copy(newSlice, slice[i:])
			return newSlice
		}
	}
	// Если элемент не найден, возвращаем пустой слайс
	return []string{}
}

func convertMapToSlice(cycles map[int][]string) [][]string {
	// Создаем двумерный слайс нужного размера
	result := make([][]string, 0, len(cycles))

	// Проходим по всем элементам карты и добавляем их в слайс
	for _, cycle := range cycles {
		result = append(result, cycle)
	}

	return result
}

func hashcode(slice []string) uint32 {
	// Соединяем все строки в одну большую строку
	combined := strings.Join(slice, ",")

	// Вычисляем хеш-код с использованием crc32
	hash := crc32.ChecksumIEEE([]byte(combined))

	return hash
}

// Вспомогательная функция DFS для поиска цикла
func dfs(graph map[string][]string, node string, visited map[string]struct{}, path []string, cycles map[int][]string) {
	visited[node] = struct{}{}
	path = append(path, node)
	for _, neighbor := range graph[node] {
		if _, ok := visited[neighbor]; !ok {
			dfs(graph, neighbor, visited, path, cycles)
		} else if slices.Contains(path, neighbor) {
			// cycle := append(removeElementsBefore(path, neighbor), neighbor)
			cycle := removeElementsBefore(path, neighbor)
			cycles[int(hashcode(cycle))] = cycle
		}
	}
}

// Helper function to perform topological sort on the graph
func topologicalSort(graph map[string][]string) ([]string, error) {
	inDegree := make(map[string]int)
	for node := range graph {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, neighbor := range graph[node] {
			inDegree[neighbor]++
		}
	}

	var zeroInDegree []string
	for node, degree := range inDegree {
		if degree == 0 {
			zeroInDegree = append(zeroInDegree, node)
		}
	}

	var result []string
	for len(zeroInDegree) > 0 {
		node := zeroInDegree[0]
		zeroInDegree = zeroInDegree[1:]
		result = append(result, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				zeroInDegree = append(zeroInDegree, neighbor)
			}
		}
	}

	return result, nil
	// //if len(result) == len(inDegree) {
	// 	return result, nil
	// }
	// return nil, fmt.Errorf("graph has a cycle or is disconnected")
}

func topologicalSortAndPrint(graph map[string][]string) {

	sorted, err := topologicalSort(graph)
	if err != nil {
		// log.Fatal(err)
	}

	file, err := os.Create("graph.dot")
	if err != nil {
		// log.Fatal(err)
	}
	defer file.Close()

	fmt.Fprintln(file, "digraph G {")
	fmt.Fprintln(file, "rankdir=TB;")
	fmt.Fprintln(file, "splines=ortho;")
	fmt.Fprintln(file, "concentrate=true;")

	nodeRanks := make(map[string]int)
	for rank, node := range sorted {
		nodeRanks[node] = rank
	}

	for node, rank := range nodeRanks {
		fmt.Fprintf(file, "{rank=%d; \"%s\";}\n", rank, node)
	}

	for from, tos := range graph {
		for _, to := range tos {
			fmt.Fprintf(file, "\"%s\" -> \"%s\";\n", from, to)
		}
	}

	fmt.Fprintln(file, "}")
}

func pprintMatrix(matrix [][]string, filename string) error {
	// Открываем файл для записи, создаем его если не существует
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, row := range matrix {
		fmt.Fprintf(file, "[")
		for i, elem := range row {
			if i != len(row)-1 {
				fmt.Fprintf(file, "%s, ", elem)
			} else {
				fmt.Fprintf(file, "%s", elem)
			}
		}
		fmt.Fprintln(file, "]")
	}
	return nil
}

// Функция для проверки, являются ли два списка дубликатами сдвигов вправо
func isDuplicateWithShift(list1, list2 []string) bool {
	if len(list1) != len(list2) {
		return false
	}

	// Создаем строку, состоящую из list1 дважды, чтобы проверить подстроку
	doubleList1 := strings.Join(list1, ",") + "," + strings.Join(list1, ",")
	joinedList2 := strings.Join(list2, ",")

	return strings.Contains(doubleList1, joinedList2)
}

// Функция для удаления дублирующих списков из [][]string
func removeDuplicates(lists [][]string) [][]string {
	uniqueLists := [][]string{}

	for i := 0; i < len(lists); i++ {
		isDuplicate := false

		for j := 0; j < len(uniqueLists); j++ {
			if isDuplicateWithShift(lists[i], uniqueLists[j]) {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			uniqueLists = append(uniqueLists, lists[i])
		}
	}

	sort.Slice(uniqueLists, func(i, j int) bool {
		return len(uniqueLists[i]) < len(uniqueLists[j])
	})
	return uniqueLists
}

func (app *App) PrintGraph() {
	graph := map[string][]string{}
	nogo := []string{
		// "anytype-heart", //todo
		"anytype-heart/core/block.Service",
		"anytype-heart/core/anytype/config.Config",
		"anytype-heart/core/wallet.wallet",
		"anytype-heart/core/block.Service",
		"anytype-heart/pkg/lib/localstore/objectstore.dsObjectStore",
		"anytype-heart/core/event.GrpcSender",
	}
	for key, values := range app.getOrder() {
		for _, value := range values {
			contains := false
			for _, ng := range nogo {
				// todo
				if strings.Contains(key, ng) || strings.Contains(value, ng) {
					contains = true
					break
				}
			}
			if !contains {
				// strings.Contains(key, "any-sync") ||
				if strings.Contains(key, "anytype-heart") && strings.Contains(value, "any-sync") {
					graph[key] = append(graph[key], value)
				}
			}
		}
	}

	cycles, found := findCycles(graph)
	cycles = removeDuplicates(cycles)

	if found {
		fmt.Println("циклы")
		pprintMatrix(cycles, "cycles.txt")
		pprintMatrixDotToFile(cycles, "cycles.dot")
	}

	topologicalSortAndPrint(graph)
}

func (app *App) initPrivateComponents() (err error) {
	for _, s := range app.privateComponents {
		if err = s.Init(app); err != nil {
			// closeServices(i) todo
			return fmt.Errorf("can't init priv service '%s': %w", s.Name(), err)
		}
	}
	return nil
}

func (app *App) initPublicComponents() (err error) {
	for _, s := range app.components {
		if err = s.Init(app); err != nil {
			// closeServices(i) todo
			return fmt.Errorf("can't init service '%s': %w", s.Name(), err)
		}
	}
	return nil
}

func (app *App) init() {
	app.initPrivateComponents()
	app.initPublicComponents()
	for _, s := range app.scopes {
		s.init()
	}
}

func (app *App) runPrivateComponents(ctx context.Context) (err error) {
	for _, s := range app.privateComponents {
		if serviceRun, ok := s.(ComponentRunnable); ok {
			start := time.Now()
			if err = serviceRun.Run(ctx); err != nil {
				// closeServices(i) todo
				// return fmt.Errorf("can't run service '%s': %w", serviceRun.Name(), err)
			}
			spent := time.Since(start).Milliseconds()
			app.startStat.SpentMsTotal += spent
			// app.startStat.SpentMsPerComp[s.Name()] = spent restore
		}
	}
	return nil
}

func (app *App) runPublicComponents(ctx context.Context) (err error) {
	for _, s := range app.components {
		if serviceRun, ok := s.(ComponentRunnable); ok {
			start := time.Now()
			if err = serviceRun.Run(ctx); err != nil {
				// closeServices(i) todo
				return fmt.Errorf("can't run service '%s': %w", serviceRun.Name(), err)
			}
			spent := time.Since(start).Milliseconds()
			app.startStat.SpentMsTotal += spent
			// app.startStat.SpentMsPerComp[s.Name()] = todo spent restore
		}
	}
	return nil
}

func (app *App) runScopes(ctx context.Context) {
	app.runPrivateComponents(ctx) // todo error handling
	app.runPublicComponents(ctx)  // todo error handling
	for _, s := range app.scopes {
		s.runScopes(ctx)
	}
}

func (app *App) injectModules() {
	for _, s := range app.privateComponents {
		if module, ok := s.(AppModule); ok {
			module.Inject(app)
		}
	}
	for _, s := range app.components {
		if module, ok := s.(AppModule); ok {
			module.Inject(app)
		}
	}
	for _, s := range app.scopes {
		s.injectModules()
	}
}

func pprintMatrixDotToFile(matrix [][]string, filename string) error {
	// Открываем файл для записи, создаем его если не существует
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Используем map для отслеживания добавленных ребер
	edges := make(map[string]bool)

	// Записываем начало графа в файл
	_, err = file.WriteString("digraph G {\n")
	fmt.Fprintln(file, "rankdir=TB;")
	fmt.Fprintln(file, "splines=ortho;")
	fmt.Fprintln(file, "concentrate=true;")
	if err != nil {
		return err
	}

	for _, cycle := range matrix {
		for i := 0; i < len(cycle); i++ {
			from := cycle[i]
			to := cycle[(i+1)%len(cycle)]
			edge := fmt.Sprintf("\"%s\" -> \"%s\"", from, to)

			if !edges[edge] {
				edges[edge] = true
				_, err := file.WriteString("    " + edge + ";\n")
				if err != nil {
					return err
				}
			}
		}
	}

	// Записываем конец графа в файл
	_, err = file.WriteString("}\n")
	if err != nil {
		return err
	}

	return nil
}
