# go-anytype-infrastructure-experiments
This repository will have the code for new infrastructure client and node prototypes

## Project structure
- **app** - DI, loggers, common engine
- **bin** - contains compiled binaries (under gitignore)
- **cmd** - main files by directories
- **config** - config component
- **etc** - default/example config files, keys, etc
- **service** - services, runtime components (these packages can use code from everywhere)
- **pkg** - some static packages that can be able to move to a separate repo, dependencies of these packages limited to this folder (maybe util)
- **util** - helpers